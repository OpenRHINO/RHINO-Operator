/*
Copyright 2022.

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

package v1alpha2

import (
	rhinojobv1alpha1 "github.com/OpenRHINO/RHINO-Operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// The ObjectMeta and Status fields are identical between v1alpha1 and v1alpha2,
// but the Spec fields are different.
func (src *RhinoJob) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*rhinojobv1alpha1.RhinoJob)

	// ObjectMeta
	dst.ObjectMeta = src.ObjectMeta

	// Spec
	dst.Spec.Image = src.Spec.Image
	dst.Spec.Parallelism = src.Spec.Parallelism
	dst.Spec.TTL = src.Spec.TTL
	dst.Spec.AppExec = src.Spec.AppExec
	dst.Spec.AppArgs = src.Spec.AppArgs
	dst.Spec.DataServer = src.Spec.DataServer
	dst.Spec.DataPath = src.Spec.DataPath

	// Status
	dst.Status.JobStatus = rhinojobv1alpha1.JobStatus(src.Status.JobStatus)

	return nil
}

func (dst *RhinoJob) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*rhinojobv1alpha1.RhinoJob)

	// ObjectMeta
	dst.ObjectMeta = src.ObjectMeta

	// Spec
	dst.Spec.Image = src.Spec.Image
	dst.Spec.Parallelism = src.Spec.Parallelism
	dst.Spec.TTL = src.Spec.TTL
	dst.Spec.AppExec = src.Spec.AppExec
	dst.Spec.AppArgs = src.Spec.AppArgs
	dst.Spec.DataServer = src.Spec.DataServer
	dst.Spec.DataPath = src.Spec.DataPath

	// Status
	dst.Status.JobStatus = JobStatus(src.Status.JobStatus)

	return nil
}
