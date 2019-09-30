FROM golang:1.13
WORKDIR /work
RUN apt-get update && \
    apt-get install iproute2 iputils-ping vim -y
COPY code code
COPY keys keys
COPY configs configs
COPY include/make_dep.sh .
RUN /work/make_dep.sh gitlab.com/alephledger/consensus-go /work/code
COPY include/run.sh .
CMD /bin/bash