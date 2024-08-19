#!/bin/bash

# 检查是否启动了etcd
if [ "$(docker ps -q -f name=etcd)" ]; then
    echo "Stopping and removing existing etcd container..."
    docker stop etcd
    docker rm etcd
else
    echo "No existing etcd container found."
fi

# 检查是否启动了redis
if [ "$(docker ps -q -f name=redis)" ]; then
    echo "Stopping and removing existing redis container..."
    docker stop redis
    docker rm redis
else
    echo "No existing redis container found."
fi

docker-compose up -d
echo "New redis container started."