/*

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

package controllers

import (
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"golang.org/x/net/context"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	batchv1 "github.com/kfyharukz/kubebuilder-tutorial/api/v1"
	kbatch "k8s.io/api/batch/v1"
)

func testCronjobReconcile() {
	var c client.Client
	var stopMgr chan struct{}
	var mgrStopped *sync.WaitGroup

	const timeout = time.Second * 5

	It("should setup Manager&Reconciler", func() {
		mgr, err := manager.New(cfg, manager.Options{})
		Expect(err).NotTo(HaveOccurred())
		c = mgr.GetClient()

		rc := &CronJobReconciler{
			Client: mgr.GetClient(),
			Log:    ctrl.Log.WithName("controllers").WithName("Captain"),
			Scheme: mgr.GetScheme(),
		}

		err = ctrl.NewControllerManagedBy(mgr).
			For(&batchv1.CronJob{}).
			Owns(&kbatch.Job{}).
			Complete(rc)
		Expect(err).NotTo(HaveOccurred())

		stopMgr, mgrStopped = StartTestManager(mgr)
	})

	It("should create CronJob", func() {
		instance := &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default"}}
		err := c.Create(context.TODO(), instance)
		Expect(apierrors.IsInvalid(err)).Should(BeFalse(), "%v", err)
		Expect(err).NotTo(HaveOccurred())

		defer c.Delete(context.TODO(), instance)
		var actual batchv1.CronJob
		Eventually(func() error {
			err := c.Get(context.TODO(), types.NamespacedName{Name: "foo", Namespace: "default"}, &actual)
			if err != nil {
				return err
			}
			return nil
		}).Should(Succeed())
	})

	It("should stop Manager", func() {
		close(stopMgr)
		mgrStopped.Wait()
	})
}
