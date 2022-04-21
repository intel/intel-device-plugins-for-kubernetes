#!/bin/bash

RTMP_SERVER=svc-nginx-rtmp
RTMP_ENDPOINT='live'
STREAM_NAME='my-video'
VARIANT_HI_MAXRATE=6000k
VARIANT_HI_VWIDTH=1920
VARIANT_LO_MAXRATE=3000k
VARIANT_LO_VWIDTH=1280

function streaming_file {
    local file=$1
    echo "Start streaming $file ..."

    local render_device=''
    local vendor_id=''
    for i in $(seq 128 255); do
        if [ -c "/dev/dri/renderD$i" ]; then
            vendor_id=$(cat "/sys/class/drm/renderD$i/device/vendor")
            if [ "$vendor_id" = '0x8086' ]; then
                # Intel GPU found
                render_device="/dev/dri/renderD$i"
                break
            fi
        fi
    done

    local o_hwaccel=''
    local o_audio='-map a:0 -c:a aac -ac 2'
    local o_decode='-c:v h264'
    local o_encode='-c:v libx264'
    local o_scaler='-vf scale'
    if [ "$render_device" != "" ]; then
        # Use hardware codec if available
        o_hwaccel="-hwaccel qsv -qsv_device $render_device"
        o_decode='-c:v h264_qsv'
        o_encode='-c:v h264_qsv'
        o_scaler='-vf scale_qsv'
    fi

    local o_variant_hi="-maxrate:v $VARIANT_HI_MAXRATE"
    local o_variant_lo="-maxrate:v $VARIANT_LO_MAXRATE"
    local width=''
    width=$(ffprobe -v error -select_streams v:0 -show_entries stream=width -of default=nw=1 "$file")
    width=${width/width=/}
    if [ "$width" -gt "$VARIANT_HI_VWIDTH" ]; then
        # Scale down to FHD for variant-HI
        o_variant_hi+=" $o_scaler=$VARIANT_HI_VWIDTH:-1"
    fi
    if [ "$width" -gt "$VARIANT_LO_VWIDTH" ]; then
        # Scale down to HD for variant-LO
        o_variant_lo+=" $o_scaler=$VARIANT_LO_VWIDTH:-1"
    fi

    eval ffmpeg -re "$o_hwaccel $o_decode -i $file" \
                -map v:0 "$o_variant_hi $o_encode $o_audio" \
                -f flv "rtmp://$RTMP_SERVER/$RTMP_ENDPOINT/$STREAM_NAME\_hi" \
                -map v:0 "$o_variant_lo $o_encode $o_audio" \
                -f flv "rtmp://$RTMP_SERVER/$RTMP_ENDPOINT/$STREAM_NAME\_lo"
}

ROOT=${BASH_SOURCE%/bin/*}
CLIPS=$ROOT/clips

while :
    do
        for clip in "$CLIPS"/*.mp4; do
            streaming_file "$clip"
        done
    done
