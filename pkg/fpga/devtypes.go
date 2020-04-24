// Copyright 2020 Intel Corporation. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fpga

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/pkg/errors"
)

// GetAfuDevType returns extended resource name for AFU without namespace.
// Since in Linux unix socket addresses can't be longer than 108 chars we need
// to compress devtype a bit, because it's used as a part of the socket's address.
// Also names of extended resources (without namespace) cannot be longer than 63 characters.
func GetAfuDevType(interfaceID, afuID string) (string, error) {
	bin, err := hex.DecodeString(interfaceID + afuID)
	if err != nil {
		return "", errors.Wrapf(err, "failed to decode %q and %q", interfaceID, afuID)
	}

	return fmt.Sprintf("af-%s.%s.%s", interfaceID[:3], afuID[:3], base64.RawURLEncoding.EncodeToString(bin)), nil
}
