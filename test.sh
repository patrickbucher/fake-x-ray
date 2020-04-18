#!/usr/bin/env bash

for i in {1..1000}
do
    curl -X POST localhost:8080/score -d @example.txt >/dev/null 2>/dev/null
    echo $i
done
