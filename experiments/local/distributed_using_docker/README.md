# docker installation and configuration (ubuntu/systemd specific)

1. `sudo apt install docker.io`
2. add yourself to docker group: `sudo usermod -aG docker <username>`
3. force docker to use "user namespaces", i.e. not runnig containers as your local root, see <https://docs.docker.com/engine/security/userns-remap/>:
    ```
    sudo systemctl edit docker

    [Service]

    ExecStart=

    ExecStart=/usr/bin/dockerd --userns-remap="<username>:<username>" -H fd://
    ```
4. restart your machine, so you can see yourself in the docker group


# Steps to create a distributed version of a test using docker services.

You can execute docker_init.sh or execute manually the following instructions. For your convenience we also included a script that tries to clean after a test.

1. start docker in swarm mode: `docker swarm init`; and add workers: `docker swarm join --token SWMTKN-1-1rzjviab1f74lm8xdo3n0lg01g2q8tqa0xrn8hcg6yy2jqym3c-8nge5n7yru9avzwx7go4thyz1 192.168.5.173:2377` (change token); verify: `docker node ls`
2. `docker network create -d overlay --attachable aleph_net`
3. create folder `code` - its structure should be similar to your GOPATH, i.e. src/gitlab.com/alephledger/consensus-go - just our project
4. create new configuration or use the default one (folders `keys` and `configs`) - it will be embedded in the docker image that you are going to build
5. start image registry: `docker service create --name registry --publish published=5000,target=5000 registry:2` (your docker image will be stored in it)
6. build image: `docker build -t localhost:5000/aleph:test .`
7. push image to local registry: `docker push localhost:5000/aleph:test`
8. ready to go: execute ./spawn.sh. It starts all (8) services

If you want to enforce a worker on which a service should be executed, modify spawn.sh or execute manually `docker service create...` with parameter `--constraint node.role==worker`.

Default names for nodes: node_1, node_2,... remember to generate keys and committee.ka using these names. Then you need to rebuild the docker image.

## Troubleshooting:
- docker service ls
- docker service ps node_1
- docker service rm node_1
- copying logs from containers:
  ```
  docker cp <container_id>:/extract .
  ```


# How to spawn a distributed test capable of simulating network latency etc.?

1. Create overlay network between swarm workers: `docker network create -d overlay --attachable aleph_net`.
   This way all the instances will behave like they are in a single network.
2. Don't use services or docker-compose or stack deploy, since they are not supporting the `cap_add NET_ADMIN` option and assigns random names to nodes/replicas.
3. Prepare images based on your aleph's configuration.
4. Run several instances on available machines: `docker run -ti --name node_4 --hostname node_4 --network aleph_net aleph:test /work/run.sh 3 /work/code /work/keys /work/configs g`.
