#!/bin/bash
##############################################################
# Basic system info script(cpu, memory, disk)
##############################################################

export LANG=C
OPT_IO=0
for opt in "${@}"; do
  case ${opt} in
  --iostat)
    OPT_IO=1
    ;;
  -h|--help)
    cat <<EOS
Usage: sysinfo.sh [options]
Options:
    -h,--help: show this message.
    --iostat: show with io device utilizations
Available metrics:
    CPU_USAGE: CPU utilization in %.
    LOAD_AVERAGE_1
    LOAD_AVERAGE_5
    LOAD_AVERAGE_15
    MEM_USAGE: Used memory in %.
    SWAP_USAGE: Used swap in %.
    DISK_USAGE_{1) : {1} is a mount point name(i.e. /home). Used disk space in %.
    DEV_RKBS_{1}: {1} is a device name(i.e. sda). Read N KBytes per second within last 15 seconds.
    DEV_WKBS_{1}: {1} is a device name(i.e. sda). Wrote N KBytes per second within last 15 seconds.
    DEV_UTIL_{1}: {1} is a device name(i.e. sda). Device utilization in %.
EOS
    exit 1
  ;;
  esac
done



FREE=`free`
TOTAL_MEM=`echo "${FREE}" | grep Mem | awk '{print $2}'`
UNUSED_MEM=`echo "${FREE}" | grep cache: | awk '{print $4}'`
USED_MEM=`expr ${TOTAL_MEM} - ${UNUSED_MEM}`
MEM_USAGE=`echo ${USED_MEM} ${TOTAL_MEM} | awk '{ print int($1 / $2 * 100) }'`

TOTAL_SWAP=`echo "${FREE}" | grep Swap | awk '{print $2}'`
USED_SWAP=`echo "${FREE}" | grep Swap | awk '{print $3}'`
SWAP_USAGE=`echo ${USED_SWAP} ${TOTAL_SWAP} | awk '{ print int($1 / $2 * 100) }'`
if [ ${TOTAL_SWAP} -eq 0 ]; then
  SWAP_USAGE=0
fi


TOP=`top -b -n1`
CPU_USAGE=`echo "${TOP}" | sed -e s/%// | sed -e s/us,//  | grep -E '^Cpu' | awk '{print int($2)}'`
LOAD_AVERAGES=(`echo "${TOP}" | grep 'load average' | awk '{gsub(/,/, "")}{i=NF-2;j=NF-1; print $i " " $j " " $NF}'`)

echo -n "CPU_USAGE:${CPU_USAGE}"
echo -ne "\t"
echo -n "LOAD_AVERAGE_1:${LOAD_AVERAGES[0]}"
echo -ne "\t"
echo -n "LOAD_AVERAGE_5:${LOAD_AVERAGES[1]}"
echo -ne "\t"
echo -n "LOAD_AVERAGE_15:${LOAD_AVERAGES[2]}"
echo -ne "\t"
echo -n "MEM_USAGE:${MEM_USAGE}"
echo -ne "\t"
echo -n "SWAP_USAGE:${SWAP_USAGE}"
OLD_IFS=$IFS
IFS=$'\n'
DF_LINES=(`df -P`)
IFS=${OLD_IFS}
i=0
for LINE in "${DF_LINES[@]}"; do
  if [ $i -eq 0 ]; then
    i=`expr $i + 1`
  else
    FILE_SYSTEM=`echo "${LINE}" | awk '{print $1}'`
    if [ "${FILE_SYSTEM}" != "none" ]; then
      DISK_USAGE=`echo "${LINE}" | sed -e s/%// | awk '{print $5}'`
      MOUNT_NAME=`echo "${LINE}" | awk '{print $6}'`
      echo -ne "\t"
      echo -n "DISK_USAGE_${MOUNT_NAME}:${DISK_USAGE}"
    fi
  fi
done

if [ ${OPT_IO} -eq 1 ]; then
  which iostat > /dev/null 2>&1
  if [ $? -eq 0 ]; then
    OLD_IFS=$IFS
    IFS=$'\n'
    IOSTAT_LINES=(`iostat -xk 15 2 | awk 'BEGIN {count=0} /^Device/ { count = count+1 } count > 1 { print $0 }' | grep -v Device`)
    IFS=${OLD_IFS}
    for LINE in "${IOSTAT_LINES[@]}"; do
      DEVICE_NAME=`echo "${LINE}" | awk '{print $1}'`
      DEVICE_RKBS=`echo "${LINE}" | awk '{print $6}'`
      DEVICE_WKBS=`echo "${LINE}" | awk '{print $7}'`
      DEVICE_UTIL=`echo "${LINE}" | awk '{print $NF}'`
      echo -ne "\t"
      echo -n "DEV_RKBS_${DEVICE_NAME}:${DEVICE_RKBS}"
      echo -ne "\t"
      echo -n "DEV_WKBS_${DEVICE_NAME}:${DEVICE_WKBS}"
      echo -ne "\t"
      echo -n "DEV_UTIL_${DEVICE_NAME}:${DEVICE_UTIL}"
    done
  fi
fi
echo
