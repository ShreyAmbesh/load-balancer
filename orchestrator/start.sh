#!/bin/bash

BUILD_IMAGES=true
RUN_ORCHESTRATOR_DIRECTLY=true
MAX_ORCHESTRATOR_INSTANCES=5

for arg in "$@"
do
    if [ "$arg" == "--no-build" ] || [ "$arg" == "--nb" ]
    then
        BUILD_IMAGES=false
        break
    fi
    if [ "$arg" == "--do" ] || [ "$arg" == "--distributed-orchestrator" ]
    then
        RUN_ORCHESTRATOR_DIRECTLY=false
    fi
done

if [ "$BUILD_IMAGES" = true ]
then
    cd ../load-balancer-server || exit
    docker rmi load-balancer-server
    docker build --tag load-balancer-server .
    cd ../backend-server || exit
    docker rmi backend-server
    docker build --tag backend-server .
    cd ../orchestrator || exit
fi
go build -o orchestrator-server
if [ "$RUN_ORCHESTRATOR_DIRECTLY" = false ]
then
  echo "Running orchestrator directly"
  for ((i=0; i<MAX_ORCHESTRATOR_INSTANCES; i++))
  do
        echo "Running orchestrator $i"
        RUNNER_PORT=$((3010 + i*10))
        NEIGHBOUR_PORT=$((3020  + i*10))
        IS_LEADER=false
        if [ $i -eq $((MAX_ORCHESTRATOR_INSTANCES - 1)) ]
        then
            NEIGHBOUR_PORT=3010
            IS_LEADER=true
         fi
        RUNNER_PORT=$RUNNER_PORT NEIGHBOUR_PORT=$NEIGHBOUR_PORT MAX_NODES=$MAX_ORCHESTRATOR_INSTANCES IS_LEADER=$IS_LEADER ./orchestrator-server &
  done
else
    ./orchestrator-server &
fi

function trap_ctrlc ()
{
    # perform cleanup here
    printf "\nPerforming clean up"

    printf "\nDoing cleanup\n"
    pkill -f orchestrator-server
    docker stop $(docker ps -a -q --filter="name=lb-service")
    docker rm $(docker ps -a -q --filter="name=lb-service")
    docker network rm load-balancer-network

    # exit shell script with error code 2
    # if omitted, shell script will continue execution
    exit 2
}

# initialise trap to call trap_ctrlc function
# when signal 2 (SIGINT) is received
trap "trap_ctrlc" 2

# your script goes here
echo "Waiting to exit"
sleep 50000
trap_ctrlc
echo "Exiting..."