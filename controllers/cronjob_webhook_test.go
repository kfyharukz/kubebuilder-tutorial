package controllers

import (
	"context"
	"crypto/tls"
	"io/ioutil"
	"net"
	"strings"
	"sync"
	"time"

	"sigs.k8s.io/yaml"

	admissionregistration "k8s.io/api/admissionregistration/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	batchv1 "github.com/kfyharukz/kubebuilder-tutorial/api/v1"
)

func testCronjobWebhook() {
	var c client.Client
	var stopMgr chan struct{}
	var mgrStopped *sync.WaitGroup

	const timeout = time.Second * 5
	const manifests = "../config/webhook/manifests.yaml"

	It("should setup Manager&Webhook", func() {
		mgr, err := manager.New(cfg, manager.Options{})
		Expect(err).NotTo(HaveOccurred())
		c = mgr.GetClient()

		err = (&batchv1.CronJob{}).SetupWebhookWithManager(mgr)
		Expect(err).NotTo(HaveOccurred())
		mgr.GetWebhookServer().CertDir = "certs"
		mgr.GetWebhookServer().Port = 30443

		stopMgr, mgrStopped = StartTestManager(mgr)
		d := &net.Dialer{Timeout: time.Second}
		Eventually(func() error {
			conn, err := tls.DialWithDialer(d, "tcp", "127.0.0.1:30443", &tls.Config{
				InsecureSkipVerify: true,
			})
			if err != nil {
				return err
			}
			conn.Close()
			return nil
		}).Should(Succeed())

		err = installWebhooks(manifests, "certs/ca.crt", c)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should create CronJob", func() {
		instance := &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default"}}
		err := c.Create(context.TODO(), instance)
		Expect(apierrors.IsInvalid(err)).Should(BeFalse(), "%v", err)
		Expect(err).NotTo(HaveOccurred())

		var actual batchv1.CronJob
		c.Get(context.TODO(), types.NamespacedName{Name: "foo", Namespace: "default"}, &actual)

		defer c.Delete(context.TODO(), instance)
	})

	It("should stop Manager", func() {
		close(stopMgr)
		mgrStopped.Wait()
	})
}

func installWebhooks(manifest string, cacert string, c client.Client) error {
	caBundle, err := ioutil.ReadFile(cacert)
	Expect(err).ShouldNot(HaveOccurred())

	mwc, vwc := decodeAndEditWebhookConfiguration(manifest, caBundle)

	err = c.Create(context.TODO(), mwc)
	Expect(err).ShouldNot(HaveOccurred())
	err = c.Create(context.TODO(), vwc)
	Expect(err).ShouldNot(HaveOccurred())

	return nil
}

func decodeAndEditWebhookConfiguration(manifests string, caBundle []byte) (*admissionregistration.MutatingWebhookConfiguration, *admissionregistration.ValidatingWebhookConfiguration) {
	input, err := ioutil.ReadFile(manifests)
	Expect(err).ShouldNot(HaveOccurred())
	strManifests := strings.Split(string(input), "---")
	mwhStr := strManifests[1]
	vwhStr := strManifests[2]

	var mwh admissionregistration.MutatingWebhookConfiguration
	yaml.Unmarshal([]byte(mwhStr), &mwh)
	mwhURLStr := "https://127.0.0.1:30443" + *mwh.Webhooks[0].ClientConfig.Service.Path
	mwh.Webhooks[0].ClientConfig.Service = nil
	mwh.Webhooks[0].ClientConfig.CABundle = caBundle
	mwh.Webhooks[0].ClientConfig.URL = &mwhURLStr

	var vwh admissionregistration.ValidatingWebhookConfiguration
	yaml.Unmarshal([]byte(vwhStr), &vwh)
	vwhURLStr := "https://127.0.0.1:30443" + *vwh.Webhooks[0].ClientConfig.Service.Path
	vwh.Webhooks[0].ClientConfig.Service = nil
	vwh.Webhooks[0].ClientConfig.CABundle = caBundle
	vwh.Webhooks[0].ClientConfig.URL = &vwhURLStr

	return &mwh, &vwh
}
