#!/bin/bash

# while getopts ":mod:" opt; do
#   case $opt in
#     mod)
#       number="$OPTARG"
#       ;;
#     \?)
#       echo "Invalid option: -$OPTARG" >&2
#       exit 1
#       ;;
#     :)
#       echo "Option -$OPTARG requires an argument." >&2
#       exit 1
#       ;;
#   esac
# done

rm ./log/*.txt

go run ./cmd/health_checker/main.go --port ":5100" > ./log/health_checker.txt 2>&1 & #port:6100
go run ./cmd/routing_server/main.go --port ":5200" > ./log/routing_server.txt 2>&1 & #port:6200
go run ./cmd/routing_watcher/main.go > ./log/routing_watcher.txt 2>&1 & #port:6300

# case $number in
#   1)
#     go test -v ./cmd/consumer/consistent_hash_test.go -timeout 1h
#     echo "Executing consistent hash"
#     ;;
#   2)
#     go test -v ./cmd/consumer/key_routing_test.go -timeout 1h
#     echo "Executing key routing"
#     ;;
#   3)
#     echo "Executing provider offline"
#     ;;
#   4)
#     echo "Executing operation 4"
#     ;;
#   *)
#     echo "Invalid operation number. Please choose between 1, 2, 3, and 4."
#     exit 1
#     ;;
# esac