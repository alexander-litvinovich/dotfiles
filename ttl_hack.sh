#!/bin/sh
sysctl -w net.inet.ip.ttl=65
touch /etc/sysctl.conf
echo "net.inet.ip.ttl=65" >> /etc/sysctl.conf
networksetup -setv6off Wi-Fi
sudo networksetup -setv6off "iPhone USB"