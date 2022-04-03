#!/bin/bash -xe

export HDDL_INSTALL_DIR=/root/hddl
export LD_LIBRARY_PATH=/root/inference_engine_samples_build/intel64/Release/lib/
/root/inference_engine_samples_build/intel64/Release/classification_sample_async -m /root/openvino_models/ir/public/squeezenet1.1/FP16/squeezenet1.1.xml -i /root/car.png -d HDDL
