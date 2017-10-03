#/bin/bash

pid1=-1
pid2=-1
pid3=-1

function finish {
  # Your cleanup code here
  pkill $pid1
  pkill $pid2
  pkill $pid3 
  rm log.out
}
trap finish EXIT

./services all --file config/file/local.json 2>&1 | tee log.out &
pid1=`echo $!`

./services api all 2>&1 | tee log.out &
pid2=`echo $!`

micro --client=grpc api --address 0.0.0.0:8081 --namespace pydio.service.api 2>&1 | tee log.out &
pid3=`echo $!`

tail -f log.out
