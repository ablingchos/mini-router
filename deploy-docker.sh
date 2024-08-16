#!/bin/bash

# 检查是否启动了etcd
if [ "$(docker ps -q -f name=etcd)" ]; then
    echo "Stopping and removing existing etcd container..."
    docker stop etcd
    docker rm etcd
else
    echo "No existing etcd container found."
fi

docker-compose up -d
echo "New etcd container started."