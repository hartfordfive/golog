#!/bin/bash


BINARY_PATH="/path/to/golog/dir"
LOG_DIR="/path/to/golog/logfiledir"
COOKIE_DOMAIN=".somedomain.com"

START_DIR=$PWD

echo "Building binary..."
/usr/bin/go build ${START_DIR}/golog.go 

if [ ! -d $BINARY_PATH ]; then
    mkdir $BINARY_PATH
fi  



PID=`ps h -o pid -C golog`
if [ "$PID" != "" ] && [ $PID -gt 0 ]; then
    echo "Killing current running golog process.."
    /bin/kill -9 $PID
fi

echo "Removing old golog binary..."
rm -f $BINARY_PATH/golog

echo "Copying binary to ${BINARY_PATH}"
cp ${START_DIR}/golog $BINARY_PATH 
rm golog

echo "Copying golog.conf to ${BINARY_PATH}"
cp sample.conf ${BINARY_PATH}/golog.conf

echo "Starting golog process..."
#nohup ${BINARY_PATH}/golog -i "" -p 8086 -d ${LOG_DIR} -db 2 -domain "${COOKIE_DOMAIN}" -ri "" -rp 8087 -stats 1 -b 5 > ${BINARY_PATH}/debug.log &
nohup ${BINARY_PATH}/golog -conf ${BINARY_PATH}/golog.conf > ${BINARY_PATH}/debug.log &

NEWPID=`ps h -o pid -C golog`
echo "GoLog now running as process ID $NEWPID"

echo -e "Install complete\n"
