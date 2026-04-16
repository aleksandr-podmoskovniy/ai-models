/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"log/slog"
	"os"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/workloadpod"
	"github.com/deckhouse/ai-models/controller/internal/bootstrap"
	"github.com/deckhouse/ai-models/controller/internal/cmdsupport"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func runManager(args []string) int {
	config, exitCode, err := parseManagerConfig(args)
	if err != nil {
		if exitCode == 2 {
			return exitCode
		}
		fmt.Fprintf(os.Stderr, "ai-models-controller: %v\n", err)
		return 1
	}

	logger, err := cmdsupport.NewComponentLogger(config.LogFormat, config.LogLevel, "controller")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ai-models-controller: %v\n", err)
		return 1
	}
	cmdsupport.SetDefaultLogger(logger)

	ctx, stop := cmdsupport.SignalContext()
	defer stop()

	workVolumeType, workVolumeSizeLimit, resources, resourceErr := runtimeResources(config)
	if resourceErr != nil {
		logger.Error(resourceErr.message, slog.Any("error", resourceErr.cause))
		return 1
	}

	application, err := bootstrap.New(logger, config.bootstrapOptions(workVolumeType, workVolumeSizeLimit, resources))
	if err != nil {
		logger.Error("unable to initialize controller app", slog.Any("error", err))
		return 1
	}

	if err := application.Run(ctx); err != nil {
		logger.Error("controller app exited with error", slog.Any("error", err))
		return 1
	}

	return 0
}

type runtimeResourcesError struct {
	message string
	cause   error
}

func runtimeResources(config managerConfig) (workVolumeType workloadpod.WorkVolumeType, workVolumeSizeLimit resource.Quantity, resources corev1.ResourceRequirements, err *runtimeResourcesError) {
	parsedWorkVolumeType, parseErr := parsePublicationWorkVolumeType(config.PublicationWorkVolumeType)
	if parseErr != nil {
		return "", resource.Quantity{}, corev1.ResourceRequirements{}, &runtimeResourcesError{
			message: "invalid publication work volume type",
			cause:   parseErr,
		}
	}

	parsedWorkVolumeSizeLimit, quantityErr := parsePositiveQuantity("publication-work-volume-size-limit", config.PublicationWorkVolumeSizeLimit)
	if quantityErr != nil {
		return "", resource.Quantity{}, corev1.ResourceRequirements{}, &runtimeResourcesError{
			message: "invalid publication work volume size limit",
			cause:   quantityErr,
		}
	}

	parsedResources, resourceErr := buildPublicationWorkerResources(
		config.PublicationWorkerCPURequest,
		config.PublicationWorkerCPULimit,
		config.PublicationWorkerMemoryRequest,
		config.PublicationWorkerMemoryLimit,
		config.PublicationWorkerEphemeralRequest,
		config.PublicationWorkerEphemeralLimit,
	)
	if resourceErr != nil {
		return "", resource.Quantity{}, corev1.ResourceRequirements{}, &runtimeResourcesError{
			message: "invalid publication worker resources",
			cause:   resourceErr,
		}
	}

	return parsedWorkVolumeType, parsedWorkVolumeSizeLimit, parsedResources, nil
}
