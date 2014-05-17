#!/bin/sh

cd /root/www_alum

exec ./www_alum 2>&1 | tee -a ./www_alum.log
