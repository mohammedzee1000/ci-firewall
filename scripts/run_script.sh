#!/usr/bin/env bash

echo "This is a sample run script. Lets begin...";
sleep 2
echo "Some secret env value FOO = $FOO1";
sleep 2
echo "Here is an IP: 192.168.122.1";
for ((i=1; i<=35; i++)); do
	echo "Counting $i of 35";
	sleep 2;
done

