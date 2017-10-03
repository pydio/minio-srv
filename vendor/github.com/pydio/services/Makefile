GOBUILD=go build
ENV=env GOOS=linux

.PHONY: all build clean start stop

all: stop build start

build:
	make -C config
	make -C datasource/objects
	make -C datasource/index
	make -C datasource/sync
	make -C tree
	make -C client

cleanall: stop clean rm

clean:
	make -C config clean
	make -C datasource/objects clean
	make -C datasource/index clean
	make -C datasource/sync clean
	make -C tree clean
	make -C client clean

start:
	docker-compose up -d
	make -C config start
	make -C datasource/objects start
	make -C datasource/index start
	make -C datasource/sync start
	make -C tree start

stop:
	docker-compose stop
	make -C config stop
	make -C datasource/objects stop
	make -C datasource/index stop
	make -C datasource/sync stop
	make -C tree stop

rm:
	docker-compose rm -f
	make -C config rm
	make -C datasource/objects rm
	make -C datasource/index rm
	make -C datasource/sync rm
	make -C tree rm
	make -C client rm
