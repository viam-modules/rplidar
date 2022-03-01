#!/bin/sh
set -o errexit

# ---- Edit based on your needs:
MONTH="Feb"
DAY="25"
YEAR="2022"

# A keyword that describes the qualities of the dataset
DESCRIPTION="viam_3rd_floor"

# This script works for osx only, and assumes the path to the rplidar is this:
RPLIDAR_PATH=/dev/tty.SLAB_USBtoUART
# If this does not apply to your case, determine the path for your device using these instructions:
# https://stackoverflow.com/questions/48291366/how-to-find-dev-name-of-usb-device-for-serial-reading-on-mac-os
# ----

DATA_DIRECTORY_NAME="data_${MONTH}_${DAY}_${YEAR}_${DESCRIPTION}"
# collect data
go run cmd/savepcdfiles/main.go -device $RPLIDAR_PATH -datafolder $DATA_DIRECTORY_NAME