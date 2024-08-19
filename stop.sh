#!/bin/bash

# 定义一个函数来停止监听特定端口的进程
stop_process_on_port() {
  local port=$1

  # 获取监听指定端口的进程的 PID
  pid=$(lsof -i:$port | awk 'NR>1 {print $2; exit}')

  # 检查是否找到了进程
  if [ -z "$pid" ]; then
    echo "No process found listening on port $port"
    return 1
  fi

  # 停止找到的进程
  echo "Killing process with PID $pid on port $port"
  kill -9 $pid

  sleep 1
  # 检查进程是否已停止
  if ! kill -0 $pid 2>/dev/null; then
    echo "Process successfully killed"
  else
    echo "Failed to kill process"
    return 1
  fi
}

# 停止监听端口 6100、6200 和 6300 的进程
stop_process_on_port 6100
stop_process_on_port 6200
stop_process_on_port 6300