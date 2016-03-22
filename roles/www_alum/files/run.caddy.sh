#!/bin/sh

cd /root/caddy
ulimit -n 4096
exec ./caddy 2>&1
