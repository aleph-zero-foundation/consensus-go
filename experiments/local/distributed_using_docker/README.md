# docker-distributed test

The intention of this test-suit is to simplify the process of executing `gomel` in a distributed manner without need of manual
synchronization of repositories and cumbersome configuration. The basic idea is that one of the participants is designated as
`swarm manager` and distributes its work/services to all available workers. The size of the network is built-in into docker
image used by this test, hence if you want to scale it, you need to generate new keys and store them inside of the keys folder
and also modify the configs/config.json file accordingly. After that you should be able to rebuild the docker image by executing
the `docker_init.sh` file on the `swarm manager`.

This test assumes there will be a single machine responsible for managing our `cluster` - a swarm manager. All other
workers must attach to it by executing `docker swarm join --token...` (see below) using a token returned by the command
`docker swarm init` executed on the swarm manager.

## docker installation and configuration (ubuntu 19.04/systemd specific)

1. `sudo apt install docker.io`
2. add yourself to docker group: `sudo usermod -aG docker <username>`
3. force docker to use `user namespaces`, i.e. not running containers as your local root user, 
   see <https://docs.docker.com/engine/security/userns-remap/> for details.
   You can achieve this by calling `dockerd` with the following additional parameter: `--userns-remap="<username>:<username>"`.
   Alternatively, you can modify docker's systemd configuration file:
    ```
    sudo systemctl edit docker

    [Service]

    ExecStart=

    ExecStart=/usr/bin/dockerd -H fd:// --containerd=/run/containerd/containerd.sock --userns-remap=<username>:<username>
    ```
4. restart your machine, so you can find yourself being a member of the `docker` system group


## Steps to create a distributed version of a test using docker services.

You can execute `docker_init.sh` or execute everything manually using the following instructions. For your convenience we also
included a script that tries to clean after each test execution, that is `docker_clean.sh`.

1. start docker in swarm mode: `docker swarm init`; and add workers: `docker swarm join --token
   SWMTKN-1-1rzjviab1f74lm8xdo3n0lg01g2q8tqa0xrn8hcg6yy2jqym3c-8nge5n7yru9avzwx7go4thyz1 192.168.5.173:2377`
   (change token to one that was output by `docker swarm init` or retrieve it on the `swarm manager` by executing
   `docker swarm join-token worker`);
   verify: `docker node ls`
2. `docker network create -d overlay --attachable aleph_net`
3. create folder `code` - its structure should be similar to your `GOPATH`, i.e. `src/gitlab.com/alephledger/consensus-go`
   ```
   mkdir -p code/src/gitlab.com/alephledger
   git clone git@gitlab.com:alephledger/consensus-go.git code/src/gitlab.com/alephledger/consensus-go
   ```
4. create a new configuration or use the default one (folders `keys` and `configs`) - it will be embedded in the docker image
   which you are going to build
5. start image registry: `docker service create --name registry --publish published=5000,target=5000 registry:2`. Your docker
   image will be stored in it.
6. build image: `docker build -t localhost:5000/aleph:test .`
7. push your image to local registry: `docker push localhost:5000/aleph:test`
8. you are ready to go: execute `./spawn.sh`. It starts all (8) services

If you want to enforce a worker on which a service should be executed, modify `spawn.sh` or execute manually 
`docker service create...` with additional parameter `--constraint node.role==worker`.

Default names for nodes: node1, node2,... remember to generate keys and committee.ka using these names. Then you need to
rebuild the docker image.

After a service finishes, you can extract its logs using the provided script `extract_logs.sh`. It expects a list of container
IDs as its argument, e.g. `docker ps -aq | xargs ./extract_logs.sh`.

In order to attach to output of one of the started services, execute: `docker service logs --raw <service_name, e.g. node1>`.

## Troubleshooting:
- docker service ls
- docker service ps node_1
- docker service rm node_1
- copying logs from containers:
  ```
  docker cp <container_id>:/extract .
  ```


## How to spawn a distributed test capable of simulating network latency etc.?

1. Create overlay network between swarm workers: `docker network create -d overlay --attachable aleph_net`. This way all the
   instances will behave like they are in a single network.
2. Don't use services or docker-compose or stack deploy, since they are not supporting the `cap_add NET_ADMIN` option and
   assigns random names to nodes/replicas.
3. Prepare images based on your aleph's configuration.
4. Run several instances on available machines: ```docker run -ti --name node_4 --hostname node_4 --network aleph_net aleph:test
   /work/run.sh 3 /work/code /work/keys /work/configs g```.
