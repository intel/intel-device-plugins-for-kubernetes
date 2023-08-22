# Copyright 2018 The TensorFlow Authors.
# Copyright 2023 Intel Corporation. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# original code from:
# https://github.com/tensorflow/examples/blob/master/courses/udacity_intro_to_tensorflow_for_deep_learning/l02c01_celsius_to_fahrenheit.ipynb
# this is slightly modified to run explicitly with XPU devices

import tensorflow as tf
import intel_extension_for_tensorflow as itex
import numpy as np

print("BACKENDS: ", str(itex.get_backend()))

devs = tf.config.list_physical_devices('XPU')

print(devs)

if not devs:
  raise Exception("No devices found")

with tf.device("/xpu:0"):
  celsius_q    = np.array([-40, -10,  0,  8, 15, 22,  38],  dtype=float)
  fahrenheit_a = np.array([-40,  14, 32, 46, 59, 72, 100],  dtype=float)

  model = tf.keras.Sequential([
    tf.keras.layers.Dense(units=1, input_shape=[1])
  ])

  model.compile(loss='mean_squared_error',
                optimizer=tf.keras.optimizers.Adam(0.1))

  history = model.fit(celsius_q, fahrenheit_a, epochs=500, verbose=False)

  print("model trained")

  test = [100.0]
  p = model.predict(test)

  if len(p) != 1:
    raise Exception("invalid result obj")

  prediction = p[0]

  if prediction >= 211 and prediction <= 213:
    print("inference ok: %f" % prediction)
  else:
    raise Exception("bad prediction %f" % prediction)

  print("SUCCESS")
