package citadel

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/rcrowley/go-metrics"
	"github.com/samalba/dockerclient"
)

var (
	ErrNoSchedulerForType = errors.New("no scheduler installed for container type")
	ErrUnableToSchedule   = errors.New("unable to schedule container")
	ErrSchedulerExists    = errors.New("scheduler exists for type")
)

// ClusterManager manages changes to the state of the cluster
type ClusterManager struct {
	engines         []*Docker
	resourceManager *ResourceManager

	schedulers map[string]Scheduler

	timer metrics.Timer

	logger *log.Logger
	mux    sync.Mutex
}

// NewClusterManager returns a new cluster manager initialized with the registry
// and a logger
func NewClusterManager(engines []*Docker, logger *log.Logger) *ClusterManager {
	m := &ClusterManager{
		engines:         engines,
		schedulers:      make(map[string]Scheduler),
		resourceManager: newDockerManger(logger),
		logger:          logger,
		timer:           metrics.NewTimer(),
	}

	metrics.Register("citadel-timer", m.timer)

	return m
}

// ScheduleContainer uses the schedulers registered with the cluster and finds
// a resource that is able to run the container.
//
// If not scheduling decision can be made an ErrUnableToSchedule error is returned.
func (m *ClusterManager) ScheduleContainer(c *Container) (*Docker, error) {
	now := time.Now()
	defer m.timer.UpdateSince(now)

	m.logger.Printf("task=%q image=%q cpus=%f memory=%f type=%q\n", "schedule", c.Image, c.Cpus, c.Memory, c.Type)

	if err := ValidateContainer(c); err != nil {
		return nil, err
	}

	m.mux.Lock()
	defer m.mux.Unlock()

	var err error

	// find the correct scheduler for the container's type
	scheduler := m.schedulers[c.Type]

	if scheduler == nil {
		return nil, ErrNoSchedulerForType
	}

	engines := m.engines

	// let the scheduler make a decision about the hosts that it would like the container to
	// be executed on
	if engines, err = scheduler.Schedule(engines, c); err != nil {
		return nil, err
	}
	m.logger.Printf("task=%q image=%q resource.count=%d\n", "schedule", c.Image, len(engines))

	// check with the resource manager to ensure that the engines that the scheduler is able
	// to run the container and to place the container on the resource with the best utilization
	// score to maximize effenciency
	placement, err := m.resourceManager.PlaceContainer(engines, c)
	if err != nil {
		return nil, err
	}
	m.logger.Printf("task=%q image=%q placement=%q\n", "schedule", c.Image, placement.Addr)

	if err := m.runContainer(c, placement); err != nil {
		return nil, err
	}

	return placement, nil
}

// RegisterScheduler registers the scheduler for a specific container type within the
// cluster.  An ErrSchedulerExists error is returned if the cluster already has a
// scheduler registered for that specific type.
func (m *ClusterManager) RegisterScheduler(tpe string, s Scheduler) error {
	m.mux.Lock()
	defer m.mux.Unlock()

	m.logger.Printf("task=%q type=%q", "register\n", tpe)

	if _, exists := m.schedulers[tpe]; exists {
		return ErrSchedulerExists
	}

	m.schedulers[tpe] = s

	return nil
}

func (m *ClusterManager) runContainer(c *Container, engine *Docker) error {
	env := []string{}
	for k, v := range c.Environment {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	config := &dockerclient.ContainerConfig{
		Hostname:   c.Hostname,
		Domainname: c.Domainname,
		Image:      c.Image,
		Memory:     int(c.Memory) * 1024 * 1024,
		Env:        env,
		CpuShares:  int(c.Cpus * 100.0 / engine.Cpus),
	}

	hostConfig := &dockerclient.HostConfig{
		PublishAllPorts: true,
	}

	if _, err := engine.client.CreateContainer(config, c.Name); err != nil {
		if err == dockerclient.ErrNotFound {
			if err := engine.client.PullImage(c.Image, "latest"); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return engine.client.StartContainer(c.Name, hostConfig)
}

func (m *ClusterManager) Remove(c *Container) error {
	m.mux.Lock()
	defer m.mux.Unlock()

	for _, engine := range m.engines {
		if err := engine.client.KillContainer(c.Name); err != nil {
			if err == dockerclient.ErrNotFound {
				continue
			}

			return err
		}

		if err := engine.client.RemoveContainer(c.Name); err != nil {
			return err
		}
	}

	return nil
}
