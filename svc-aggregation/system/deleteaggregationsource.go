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

package system

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/ODIM-Project/ODIM/lib-utilities/common"
	"github.com/ODIM-Project/ODIM/lib-utilities/errors"
	aggregatorproto "github.com/ODIM-Project/ODIM/lib-utilities/proto/aggregator"
	"github.com/ODIM-Project/ODIM/lib-utilities/response"
	"github.com/ODIM-Project/ODIM/svc-aggregation/agmodel"
)

// DeleteAggregationSource is the handler for removing  bmc or manager
func (e *ExternalInterface) DeleteAggregationSource(req *aggregatorproto.AggregatorRequest) response.RPC {
	var resp response.RPC

	aggregationSource, dbErr := agmodel.GetAggregationSourceInfo(req.URL)
	if dbErr != nil {
		log.Printf("error getting  AggregationSource : %v", dbErr)
		errorMessage := dbErr.Error()
		if errors.DBKeyNotFound == dbErr.ErrNo() {
			return common.GeneralError(http.StatusNotFound, response.ResourceNotFound, errorMessage, []interface{}{"AggregationSource", req.URL}, nil)
		}
		return common.GeneralError(http.StatusInternalServerError, response.InternalError, errorMessage, nil, nil)
	}
	// check whether the aggregation source is bmc or manager
	links := aggregationSource.Links.(map[string]interface{})
	// check links has connection method or oem
	if _, ok := links["ConnectionMethod"]; ok {
		return e.deleteAggregationSourceWithConnectionMethod(req.URL, links["ConnectionMethod"].(map[string]interface{}))
	}
	oem := links["Oem"].(map[string]interface{})
	if _, ok := oem["PluginType"]; ok {
		// Get the plugin
		pluginID := oem["PluginID"].(string)
		plugin, errs := agmodel.GetPluginData(pluginID)
		if errs != nil {
			errMsg := errs.Error()
			log.Printf(errMsg)
			return common.GeneralError(http.StatusNotFound, response.ResourceNotFound, errMsg, []interface{}{"plugin", pluginID}, nil)
		}
		// delete the manager
		resp = e.deletePlugin("/redfish/v1/Managers/" + plugin.ManagerUUID)
	} else {
		var data = strings.Split(req.URL, "/redfish/v1/AggregationService/AggregationSources/")
		systemList, dbErr := agmodel.GetAllMatchingDetails("ComputerSystem", data[1], common.InMemory)
		if dbErr != nil {
			errMsg := dbErr.Error()
			log.Println(errMsg)
			if errors.DBKeyNotFound == dbErr.ErrNo() {
				return common.GeneralError(http.StatusNotFound, response.ResourceNotFound, errMsg, []interface{}{"Systems", "everything"}, nil)
			}
			return common.GeneralError(http.StatusInternalServerError, response.InternalError, errMsg, nil, nil)
		}
		for _, systemURI := range systemList {
			index := strings.LastIndexAny(systemURI, "/")
			resp = e.deleteCompute(systemURI, index)
		}
	}
	if resp.StatusCode != http.StatusOK {
		return resp
	}
	// Delete the Aggregation Source
	dbErr = agmodel.DeleteAggregationSource(req.URL)
	if dbErr != nil {
		errorMessage := "error while trying to delete AggreationSource  " + dbErr.Error()
		resp.CreateInternalErrorResponse(errorMessage)
		log.Printf(errorMessage)
		return resp
	}

	resp = response.RPC{
		StatusCode:    http.StatusNoContent,
		StatusMessage: response.ResourceRemoved,
		Header: map[string]string{
			"Content-type":      "application/json; charset=utf-8", // TODO: add all error headers
			"Cache-Control":     "no-cache",
			"Connection":        "keep-alive",
			"Transfer-Encoding": "chunked",
			"OData-Version":     "4.0",
			"X-Frame-Options":   "sameorigin",
		},
	}
	return resp
}

func (e *ExternalInterface) deleteAggregationSourceWithConnectionMethod(url string, connectionMethodLink map[string]interface{}) response.RPC {
	var resp response.RPC
	connectionMethodOdataID := connectionMethodLink["@odata.id"].(string)
	connectionMethod, err := e.GetConnectionMethod(connectionMethodOdataID)
	if err != nil {
		log.Printf("error getting  connectionmethod : %v", err)
		errorMessage := err.Error()
		if errors.DBKeyNotFound == err.ErrNo() {
			return common.GeneralError(http.StatusNotFound, response.ResourceNotFound, err.Error(), []interface{}{"ConnectionMethod", connectionMethodOdataID}, nil)
		}
		return common.GeneralError(http.StatusInternalServerError, response.InternalError, errorMessage, nil, nil)
	}

	uuid := url[strings.LastIndexByte(url, '/')+1:]
	target, terr := agmodel.GetTarget(uuid)
	if terr != nil || target == nil {
		cmVariants := getConnectionMethodVariants(connectionMethod.ConnectionMethodVariant)
		if len(connectionMethod.Links.AggregationSources) > 1 {
			errMsg := fmt.Sprintf("error: plugin %v can't be removed since it managing some of the devices", cmVariants.PluginID)
			log.Println(errMsg)
			return common.GeneralError(http.StatusNotAcceptable, response.ResourceCannotBeDeleted, errMsg, nil, nil)
		}
		// Get the plugin
		plugin, errs := agmodel.GetPluginData(cmVariants.PluginID)
		if errs != nil {
			errMsg := errs.Error()
			log.Printf(errMsg)
			return common.GeneralError(http.StatusNotFound, response.ResourceNotFound, errMsg, []interface{}{"plugin", cmVariants.PluginID}, nil)
		}
		// delete the manager
		resp = e.deletePlugin("/redfish/v1/Managers/" + plugin.ManagerUUID)
	} else {
		var data = strings.Split(url, "/redfish/v1/AggregationService/AggregationSources/")
		systemList, dbErr := agmodel.GetAllMatchingDetails("ComputerSystem", data[1], common.InMemory)
		if dbErr != nil {
			errMsg := dbErr.Error()
			log.Println(errMsg)
			if errors.DBKeyNotFound == dbErr.ErrNo() {
				return common.GeneralError(http.StatusNotFound, response.ResourceNotFound, errMsg, []interface{}{"Systems", "everything"}, nil)
			}
			return common.GeneralError(http.StatusInternalServerError, response.InternalError, errMsg, nil, nil)
		}
		for _, systemURI := range systemList {
			index := strings.LastIndexAny(systemURI, "/")
			resp = e.deleteCompute(systemURI, index)
		}
	}
	if resp.StatusCode != http.StatusOK {
		return resp
	}
	// Delete the Aggregation Source
	dbErr := agmodel.DeleteAggregationSource(url)
	if dbErr != nil {
		errorMessage := "error while trying to delete AggreationSource  " + dbErr.Error()
		resp.CreateInternalErrorResponse(errorMessage)
		log.Printf(errorMessage)
		return resp
	}
	connectionMethod.Links.AggregationSources = removeAggregationSource(connectionMethod.Links.AggregationSources, agmodel.OdataID{OdataID: url})
	dbErr = e.UpdateConnectionMethod(connectionMethod, connectionMethodOdataID)
	if dbErr != nil {
		errMsg := dbErr.Error()
		log.Println(errMsg)
		return common.GeneralError(http.StatusInternalServerError, response.InternalError, errMsg, nil, nil)
	}

	resp = response.RPC{
		StatusCode:    http.StatusNoContent,
		StatusMessage: response.ResourceRemoved,
		Header: map[string]string{
			"Content-type":      "application/json; charset=utf-8", // TODO: add all error headers
			"Cache-Control":     "no-cache",
			"Connection":        "keep-alive",
			"Transfer-Encoding": "chunked",
			"OData-Version":     "4.0",
			"X-Frame-Options":   "sameorigin",
		},
	}
	return resp
}

// removeAggregationSource will remove the element from the slice return
// slice of remaining elements
func removeAggregationSource(slice []agmodel.OdataID, element agmodel.OdataID) []agmodel.OdataID {
	var elements []agmodel.OdataID
	for _, val := range slice {
		if val != element {
			elements = append(elements, val)
		}
	}
	return elements
}

// deleteplugin removes the given plugin
func (e *ExternalInterface) deletePlugin(oid string) response.RPC {
	var resp response.RPC
	// Get Manager Info
	data, derr := agmodel.GetResource("Managers", oid)
	if derr != nil {
		errMsg := "error while getting Managers data: " + derr.Error()
		log.Println(errMsg)
		if errors.DBKeyNotFound == derr.ErrNo() {
			return common.GeneralError(http.StatusNotFound, response.ResourceNotFound, errMsg, []interface{}{"Managers", oid}, nil)
		}
		return common.GeneralError(http.StatusInternalServerError, response.InternalError, errMsg, nil, nil)
	}
	var resource map[string]interface{}
	json.Unmarshal([]byte(data), &resource)
	var pluginID = resource["Name"].(string)
	plugin, errs := agmodel.GetPluginData(pluginID)
	if errs != nil {
		errMsg := "error while getting plugin data: " + errs.Error()
		log.Println(errMsg)
		return common.GeneralError(http.StatusNotFound, response.ResourceNotFound, errMsg, []interface{}{"Plugin", pluginID}, nil)
	}

	systems, dberr := agmodel.GetAllSystems()
	if dberr != nil {
		errMsg := derr.Error()
		log.Println(errMsg)
		if errors.DBKeyNotFound == derr.ErrNo() {
			return common.GeneralError(http.StatusNotFound, response.ResourceNotFound, errMsg, []interface{}{"Systems", "everything"}, nil)
		}
		return common.GeneralError(http.StatusInternalServerError, response.InternalError, errMsg, nil, nil)
	}
	// verifying if any device is mapped to plugin
	var systemCnt = 0
	for i := 0; i < len(systems); i++ {
		if systems[i].PluginID == pluginID {
			systemCnt++
		}
	}
	if systemCnt > 0 {
		errMsg := fmt.Sprintf("error: plugin %v can't be removed since it managing some of the devices", pluginID)
		log.Println(errMsg)
		return common.GeneralError(http.StatusNotAcceptable, response.ResourceCannotBeDeleted, errMsg, nil, nil)
	}

	// verifying if plugin is up
	var pluginContactRequest getResourceRequest

	pluginContactRequest.ContactClient = e.ContactClient
	pluginContactRequest.Plugin = plugin
	pluginContactRequest.StatusPoll = false
	pluginContactRequest.HTTPMethodType = http.MethodGet
	pluginContactRequest.LoginCredentials = map[string]string{
		"UserName": plugin.Username,
		"Password": string(plugin.Password),
	}
	pluginContactRequest.OID = "/ODIM/v1/Status"
	_, _, _, err := contactPlugin(pluginContactRequest, "error while getting the details "+pluginContactRequest.OID+": ")
	if err == nil { // no err means plugin is still up, so we can't remove it
		errMsg := "error: plugin is still up, so it cannot be removed."
		log.Println(errMsg)
		return common.GeneralError(http.StatusNotAcceptable, response.ResourceCannotBeDeleted, errMsg, nil, nil)
	}

	// deleting the manager info
	dberr = agmodel.DeleteManagersData(oid)
	if dberr != nil {
		errMsg := derr.Error()
		log.Println(errMsg)
		if errors.DBKeyNotFound == derr.ErrNo() {
			return common.GeneralError(http.StatusNotFound, response.ResourceNotFound, errMsg, []interface{}{"Managers", oid}, nil)
		}
		return common.GeneralError(http.StatusInternalServerError, response.InternalError, errMsg, nil, nil)
	}
	// deleting the plugin if  zero devices are managed
	dberr = agmodel.DeletePluginData(pluginID)
	if dberr != nil {
		errMsg := derr.Error()
		log.Println(errMsg)
		if errors.DBKeyNotFound == derr.ErrNo() {
			return common.GeneralError(http.StatusNotFound, response.ResourceNotFound, errMsg, []interface{}{"Plugin", pluginID}, nil)
		}
		return common.GeneralError(http.StatusInternalServerError, response.InternalError, errMsg, nil, nil)
	}
	e.EventNotification(oid, "ResourceRemoved", "ManagerCollection")
	resp.Header = map[string]string{
		"Cache-Control":     "no-cache",
		"Transfer-Encoding": "chunked",
		"Content-type":      "application/json; charset=utf-8",
	}
	resp.StatusCode = http.StatusOK
	resp.StatusMessage = response.ResourceRemoved

	args := response.Args{
		Code:    resp.StatusMessage,
		Message: "Request completed successfully",
	}
	resp.Body = args.CreateGenericErrorResponse()
	return resp
}

func (e *ExternalInterface) deleteCompute(key string, index int) response.RPC {
	var resp response.RPC
	// check whether the any system operation is under progress
	systemOperation, dbErr := agmodel.GetSystemOperationInfo(strings.TrimSuffix(key, "/"))
	if dbErr != nil && errors.DBKeyNotFound != dbErr.ErrNo() {
		log.Println(" Delete operation for system  ", key, " can't be processed ", dbErr.Error())
		errMsg := "error while trying to delete compute system: " + dbErr.Error()
		return common.GeneralError(http.StatusInternalServerError, response.InternalError, errMsg, nil, nil)
	}
	if systemOperation.Operation != "" {
		log.Println("Delete operation or system  ", key, " can't be processed,", systemOperation.Operation, " operation  is under progress")
		errMsg := systemOperation.Operation + " operation  is under progress"
		return common.GeneralError(http.StatusNotAcceptable, response.ResourceCannotBeDeleted, errMsg, nil, nil)
	}
	systemOperation.Operation = "Delete"
	dbErr = systemOperation.AddSystemOperationInfo(strings.TrimSuffix(key, "/"))
	if dbErr != nil {
		log.Println(" Delete operation for system  ", key, " can't be processed ", dbErr.Error())
		errMsg := "error while trying to delete compute system: " + dbErr.Error()
		return common.GeneralError(http.StatusInternalServerError, response.InternalError, errMsg, nil, nil)
	}
	defer func() {
		agmodel.DeleteSystemOperationInfo(strings.TrimSuffix(key, "/"))
	}()
	// Delete Subscription on odimra and also on device
	subResponse, err := e.DeleteEventSubscription(key)
	if err != nil && subResponse == nil {
		errMsg := fmt.Sprintf("error while trying to delete subscriptions: %v", err)
		log.Println(errMsg)
		return common.GeneralError(http.StatusInternalServerError, response.InternalError, errMsg, nil, nil)
	}
	// If the DeleteEventSubscription call return status code other than http.StatusNoContent, http.StatusNotFound.
	//Then return with error(delete event subscription failed).
	if subResponse.StatusCode != http.StatusNoContent {
		log.Println("error while deleting the event subscription for ", key, " :", subResponse.Body)
	}

	// Split the key by : (uuid:1) so we will get [uuid 1]
	k := strings.Split(key[index+1:], ":")
	if len(k) < 2 {
		errMsg := fmt.Sprintf("key %v doesn't have system details", key)
		log.Println(errMsg)
		return common.GeneralError(http.StatusInternalServerError, response.InternalError, errMsg, nil, nil)
	}
	uuid := k[0]

	chassisList, derr := agmodel.GetAllMatchingDetails("Chassis", uuid, common.InMemory)
	if derr != nil {
		log.Printf("error while trying to collect the chassis list: %v", derr)
	}

	// Delete Compute System Details from InMemory
	if derr := e.DeleteComputeSystem(index, key); derr != nil {
		errMsg := "error while trying to delete compute system: " + derr.Error()
		log.Println(errMsg)
		if errors.DBKeyNotFound == derr.ErrNo() {
			return common.GeneralError(http.StatusNotFound, response.ResourceNotFound, errMsg, []interface{}{index, key}, nil)
		}
		return common.GeneralError(http.StatusInternalServerError, response.InternalError, errMsg, nil, nil)
	}

	// Delete System Details from OnDisk
	if derr := e.DeleteSystem(uuid); derr != nil {
		errMsg := "error while trying to delete system: " + derr.Error()
		log.Println(errMsg)
		if errors.DBKeyNotFound == derr.ErrNo() {
			return common.GeneralError(http.StatusNotFound, response.ResourceNotFound, errMsg, []interface{}{"System", uuid}, nil)
		}
		return common.GeneralError(http.StatusInternalServerError, response.InternalError, errMsg, nil, nil)
	}
	for _, chassis := range chassisList {
		e.EventNotification(chassis, "ResourceRemoved", "ChassisCollection")
	}
	e.EventNotification(key, "ResourceRemoved", "SystemsCollection")
	resp.Header = map[string]string{
		"Cache-Control":     "no-cache",
		"Transfer-Encoding": "chunked",
		"Content-type":      "application/json; charset=utf-8",
	}
	resp.StatusCode = http.StatusOK
	resp.StatusMessage = response.ResourceRemoved
	args := response.Args{
		Code:    resp.StatusMessage,
		Message: "Request completed successfully",
	}
	resp.Body = args.CreateGenericErrorResponse()
	return resp
}
