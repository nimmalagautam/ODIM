//(C) Copyright [2020] Hewlett Packard Enterprise Development LP
//
//Licensed under the Apache License, Version 2.0 (the "License"); you may
//not use this file except in compliance with the License. You may obtain
//a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//License for the specific language governing permissions and limitations
// under the License.

package managers

import (
	"github.com/ODIM-Project/ODIM/lib-utilities/common"
	"github.com/ODIM-Project/ODIM/lib-utilities/errors"
	"github.com/ODIM-Project/ODIM/svc-managers/mgrcommon"
	"github.com/ODIM-Project/ODIM/svc-managers/mgrmodel"
	"github.com/ODIM-Project/ODIM/svc-plugin-rest-client/pmbhandle"
	"net/http"
)

// ExternalInterface holds all the external connections managers package functions uses
type ExternalInterface struct {
	Device Device
	DB     DB
}

// Device struct to inject the contact device function into the handlers
type Device struct {
	GetDeviceInfo         func(mgrcommon.ResourceInfoRequest) (string, error)
	ContactClient         func(string, string, string, string, interface{}, map[string]string) (*http.Response, error)
	DecryptDevicePassword func([]byte) ([]byte, error)
}

// DB struct to inject the contact DB function into the handlers
type DB struct {
	GetAllKeysFromTable func(string) ([]string, error)
	GetManagerData      func(string) (mgrmodel.RAManager, error)
	GetManagerByURL     func(string) (string, *errors.Error)
}

// GetExternalInterface retrieves all the external connections managers package functions uses
func GetExternalInterface() *ExternalInterface {
	return &ExternalInterface{
		Device: Device{
			GetDeviceInfo:         mgrcommon.GetResourceInfoFromDevice,
			ContactClient:         pmbhandle.ContactPlugin,
			DecryptDevicePassword: common.DecryptWithPrivateKey,
		},
		DB: DB{
			GetAllKeysFromTable: mgrmodel.GetAllKeysFromTable,
			GetManagerData:      mgrmodel.GetManagerData,
			GetManagerByURL:     mgrmodel.GetManagerByURL,
		},
	}
}
