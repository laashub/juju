// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package azure

import (
	"github.com/Azure/azure-sdk-for-go/arm/resources/resources"
	"github.com/juju/errors"

	"github.com/juju/juju/environs/context"
	"github.com/juju/juju/provider/azure/internal/armtemplates"
	"github.com/juju/juju/provider/azure/internal/errorutils"
)

func createDeployment(
	ctx context.ProviderCallContext,
	client resources.DeploymentsClient,
	resourceGroup string,
	deploymentName string,
	t armtemplates.Template,
) error {
	templateMap, err := t.Map()
	if err != nil {
		return errors.Trace(err)
	}
	deployment := resources.Deployment{
		&resources.DeploymentProperties{
			Template: &templateMap,
			Mode:     resources.Incremental,
		},
	}
	_, errChan := client.CreateOrUpdate(
		resourceGroup,
		deploymentName,
		deployment,
		nil, // abort channel
	)
	if err := <-errChan; err != nil {
		return errorutils.HandleCredentialError(errors.Annotatef(err, "creating deployment %q", deploymentName), ctx)
	}
	return nil
}
