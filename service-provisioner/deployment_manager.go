package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/arm/resources/resources"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/Azure/go-autorest/autorest/utils"
	"github.com/Jeffail/gabs"
	"github.com/teris-io/shortid"
)

type DeploymentManager struct {
	AppID          string
	ClientSecret   string
	TenantID       string
	SubscriptionID string
	ResourceGroup  string
	Location       string
}

type DeploymentResponse struct {
	AdminUsername          string
	AdminPassword          string
	ManagementPlaneAddress string
	NginxAddress           string
}

var (
	groupsClient               resources.GroupsClient
	deploymentsClient          resources.DeploymentsClient
	controlPlaneSku            = "Standard_B1ms"
	controlPlaneRole           = "nginxcontrolplane"
	nodesRole                  = "nginxnode"
	nodesDefaultSku            = "Standard_D1_v2"
	deploymentFilePath         = "./templates/azure_nginx_deployment.json"
	controlPlaneCustomDataPath = "./templates/controlplane_cloudinit.yaml"
	nodeCustomDataPath         = "./templates/node_cloudinit.yaml"
)

func (a *DeploymentManager) onErrorFail(err error) error {
	return err
}

func (a *DeploymentManager) initClients() {
	os.Setenv("AZURE_TENANT_ID", a.TenantID)
	os.Setenv("AZURE_SUBSCRIPTION_ID", a.SubscriptionID)
	os.Setenv("AZURE_CLIENT_ID", a.AppID)
	os.Setenv("AZURE_CLIENT_SECRET", a.ClientSecret)

	authorizer, err := utils.GetAuthorizer(azure.PublicCloud)
	if err != nil {
		panic(err)
	}

	groupsClient = resources.NewGroupsClient(a.SubscriptionID)
	groupsClient.Authorizer = authorizer

	deploymentsClient = resources.NewDeploymentsClient(a.SubscriptionID)
	deploymentsClient.Authorizer = authorizer

}

func (a *DeploymentManager) Init() {
	a.initClients()
}

func (a *DeploymentManager) CreateControlPlane(jsonParsed *gabs.Container) {
	yaml, _ := ioutil.ReadFile(controlPlaneCustomDataPath)
	ymlStr := string(yaml)

	variablesResource := jsonParsed.Search("variables")

	variablesResource.Set(ymlStr, "controlPlaneCustomData")
	variablesResource.Set(controlPlaneSku, "controlPlaneSku")
	variablesResource.Set(controlPlaneRole, "controlPlaneRole")
}

func escapeSingleLine(escapedStr string) string {
	escapedStr = strings.Replace(escapedStr, "\\", "\\\\", -1)
	escapedStr = strings.Replace(escapedStr, "\r\n", "\\n", -1)
	escapedStr = strings.Replace(escapedStr, "\n", "\\n", -1)
	escapedStr = strings.Replace(escapedStr, "\"", "\\\"", -1)
	return escapedStr
}

func (a *DeploymentManager) CreateNodes(jsonParsed *gabs.Container, sku string, nodeCount int) {
	yaml, _ := ioutil.ReadFile(nodeCustomDataPath)

	variablesResource := jsonParsed.Search("variables")

	if len(sku) == 0 {
		sku = nodesDefaultSku
	}

	variablesResource.Set(nodesDefaultSku, "nodesSku")
	variablesResource.Set(nodeCount, "nodeCount")
	variablesResource.Set(nodesRole, "nodesRole")

	ymlStr := string(yaml)
	variablesResource.Set(ymlStr, "nodeCustomData")
}

func (a *DeploymentManager) DeployNginx(deploymentName string, sku string, nodeCount int, subnetID string) (response DeploymentResponse, err error) {
	jsonParsed, err := gabs.ParseJSONFile(deploymentFilePath)
	if err != nil {
		return response, err
	}

	sid, _ := shortid.New(1, shortid.DefaultABC, 2000)
	randName, _ := sid.Generate()
	randNameStr := strings.ToLower(randName)

	a.createResourceGroup(a.ResourceGroup, a.Location)

	vnetName := "nginxvnet-" + randNameStr
	passwordSid, _ := shortid.New(1, shortid.DefaultABC, 2000)
	passwordStr, _ := passwordSid.Generate()
	passwordStr = passwordStr + "Pas@1"

	variablesResource := jsonParsed.Search("variables")
	resources := jsonParsed.Search("resources")

	adminUsername := variablesResource.Search("adminUsername").Data().(string)

	variablesResource.Set(vnetName, "virtualNetworkName")
	variablesResource.Set(passwordStr, "adminPassword")
	variablesResource.Set(deploymentName, "deploymentName")

	a.CreateControlPlane(jsonParsed)
	a.CreateNodes(jsonParsed, sku, nodeCount)

	if len(subnetID) > 0 {
		variablesResource.Set(subnetID, "subnet1Ref")
		jsonParsed.ArrayRemove(3, "resources")
		resources.Index(1).Delete("dependsOn")
	}

	deploymentJSON := jsonParsed.String()
	deployment := a.buildDeployment(deploymentJSON)
	err = a.validateDeployment(a.ResourceGroup, deploymentName, deployment)

	if err != nil {
		return response, err
	}

	depResponse := a.getDeploymentInfo(adminUsername, passwordStr, a.Location, deploymentName)

	_, errChan := deploymentsClient.CreateOrUpdate(a.ResourceGroup, deploymentName, deployment, nil)
	err = a.onErrorFail(<-errChan)

	if err != nil {
		return response, err
	}

	return depResponse, nil
}

func (a *DeploymentManager) getDeploymentInfo(adminUsername string, adminPassword string, location string, deploymentName string) DeploymentResponse {
	return DeploymentResponse{
		ManagementPlaneAddress: deploymentName + "mgmt." + location + ".cloudapp.azure.com",
		NginxAddress:           deploymentName + "." + location + ".cloudapp.azure.com",
		AdminUsername:          adminUsername,
		AdminPassword:          adminPassword,
	}
}

func (a *DeploymentManager) buildDeployment(templateJSON string) resources.Deployment {
	fileMap := map[string]interface{}{}
	json.Unmarshal([]byte(templateJSON), &fileMap)

	return resources.Deployment{
		Properties: &resources.DeploymentProperties{
			Mode:     resources.Incremental,
			Template: &fileMap,
		},
	}
}

func (a *DeploymentManager) validateDeployment(resourceGroupName string, deploymentName string, deployment resources.Deployment) error {
	_, err := deploymentsClient.Validate(resourceGroupName, deploymentName, deployment)
	return err
}

func (a *DeploymentManager) createResourceGroup(name string, location string) error {
	resourceGroupParams := resources.Group{
		Location: to.StringPtr(location),
	}

	_, err := groupsClient.CreateOrUpdate(name, resourceGroupParams)
	return err
}
