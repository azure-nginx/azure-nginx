# Azure-NGINX: Self reliant, PaaS-like nginx clusters - WIP

## Overview

azure-nginx allows you to provision highly available nginx clusters on Microsoft Azure, with all the features you'd expect from a fully managed service.
with azure-nginx you get:

* Automatic version upgrades
* Auto healing for nginx nodes
* Auto scaling
* High availability
* Custom VNET deployment
* A self reliant, RESTful API driven Control Plane
* Automatic nginx configuration sync between nodes
* Native Azure CLI integration

azure-nginx is 100% Open Source and *not* an official Azure Service.

This project is the first step towards a reliable development framework that allows for a multitude of Open Source projects to run as self reliant services on Azure.

The core components of azure-nginx will be further generalized for that purpose in the future.

## Usage

azure-nginx provisions ARM resources through the service-provisioner component, which authenticates with Azure using an SPN.

Once you have the SPN details, you can run the service-provisioner and communicate with it either through a RESTful API or the azure-nginx-cli extension.

A successful deployment will give the following output:
```
{
  "AdminPassword": "<ssh-password-for-vms>",
  "AdminUsername": "<ssh-username>",
  "ManagementPlaneAddress": "<fqdn-for-control-plane>",
  "ManagementPlaneApiToken": "<api-token-for-control-plane>",
  "NginxAddress": "<fqdn-for-nginx-servers>"
}
```

### Create an SPN

`az ad sp create-for-rbac --sdk-auth > mycredentials.json`

From the output json file you will need clientId, clientSecret, subscriptionId and tenantId.

### Run the service-provisioner using Docker

`docker run -d -p 8080:8080 -e "APP_ID=<your-app-id>" -e "CLIENT_SECRET=<your-client-secret>" -e "TENANT_ID=<your-tenant-id>" -e SUBSCRIPTION_ID="<your-subscription-id>" azurenginx/serviceprovisioner`

### Deploy an nginx cluster with 2 nodes in eastus

`curl -H "Content-Type: application/json" -d '{"name": "mynginxcluster", "nodeSku": "Standard_D1_V2", "nodeCount": 2, "resourceGroup": "my-nginx-rg", "location": "eastus"}' http://localhost:8080/nginx`

### Deploy an nginx cluster in a custom vnet

`curl -H "Content-Type: application/json" -d '{"customSubnetID": "<custom-subnet-id>" "name": "mynginxcluster", "nodeSku": "Standard_D1_V2", "nodeCount": 2, "resourceGroup": "my-nginx-rg", "location": "eastus"}' http://localhost:8080/nginx`

### Deploy using the Azure-CLI extension

The [Azure-CLI extension](https://github.com/azure-nginx/azure-nginx-cli) interacts with the service-provisioner, so make sure its running.

#### Install the cli extension: (Make sure your Azure CLI version is at least 2.0.24)

`az extension add --source 'https://github.com/azure-nginx/azure-nginx-cli/raw/master/dist/azure_nginx_cli-0.0.1-py2.py3-none-any.whl'`

#### Deploy an nginx cluster with 2 nodes in east us

`az nginx deploy --name "nginxclusterdemo" --resource-group "nginx-rg" --node-count 2 --node-sku "Standard_D1_V2" --location "eastus"`

#### deploy in a custom vnet

`az nginx deploy --custom-subnet-id "<id-of-custom-subnet>" --name "nginxclusterdemo" --resource-group "nginx-rg" --node-count 2 --node-sku "Standard_D1_V2" --location "eastus"`


## Control Plane API

The azure-nginx Control Plane is responsible for managing and upgrading the nginx nodes.
It exposes a RESTful API that allows for interaction with the control plane and the nodes.

### Authentication

The API expects a "token" header name with the value being the API token you received after your service was deployed.

### Schema

| Endpoint        | Method      | Description               |
| --------------- | ----------- | ------------------------- |
| /nodes          | GET         | Get a list of nodes       |
| /configuration  | GET         | Get nginx.conf file       |
| /configuration  | POST        | Upload an nginx.conf file |
| /upgrade        | POST        | Upgrade to latest nginx   |
| /upgrade/status | GET         | Get upgrade status        |


## How it works

![diagram](https://github.com/azure-nginx/azure-nginx/raw/master/diagram.png)








