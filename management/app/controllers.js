'use strict';

// Page header that displays the totals for the cluster
function HeaderController($scope, Hosts, Containers) {
    $scope.template = 'partials/header.html';

    Hosts.query({}, function (d) {
        $scope.hosts = d.length;
        var cpus = 0;
        var memory = 0;

        angular.forEach(d, function (v) {
            cpus += v.cpus;
            memory += v.memory;
        });

        $scope.cpus = cpus;
        $scope.memory = memory;
    });

    Containers.query({}, function (d) {
        $scope.containers = d.length;
    });
}

// Dashboard includes overall information with graphs and services 
// for the cluster
function DashboardController($scope, Hosts) {
    /*
    Host.metrics({
        name: "b8f6b1166755"
    }, function (data) {
        var mem = function (d) {
            return (d.memory.used / d.memory.total) * 100;
        };
        var cpu = function (d) {
            return d.load_1;
        };

        newAreaChart(data, mem, '#chart-memory', 'mem %');
        newAreaChart(data, cpu, '#chart-cpu', 'load 1');
    });
    */
}

function HostsController($scope, $routeParams, Hosts) {
    $scope.template = 'partials/hosts.html';

    Hosts.query({
        name: $routeParams.id
    }, function (data) {
        $scope.hosts = data;
    });
}

function ContainersController($scope, $routeParams, Containers) {
    $scope.template = 'partials/containers.html';
    $scope.predicate = '-instances';

    Containers.query({
        name: $routeParams.id
    }, function (data) {
        var groups = {};
        angular.forEach(data, function (v) {
            if (groups[v.image] === null || groups[v.image] === undefined) {
                groups[v.image] = [];
            }

            var c = groups[v.image];
            c.push(v);
        });


        var containers = [];
        angular.forEach(groups, function (v, k) {
            var cpus = 0;
            var memory = 0;

            angular.forEach(v, function (c) {
                cpus += c.cpus;
                memory += c.memory;
            });

            containers.push({
                image: k,
                instances: v.length,
                cpus: cpus || 0,
                memory: memory || 0
            });
        });

        $scope.containers = containers;
    });
}

// this needs to move to some super start init func
function toggleStartSidebar() {
    $('.ui.sidebar').sidebar({
        overlay: true
    })
        .sidebar('toggle');
}