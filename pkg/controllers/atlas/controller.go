package atlas

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"embed"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/Masterminds/sprig"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"

	"github.com/rancher/wrangler/pkg/apply"
	core "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/relatedresource"

	"github.com/ekristen/atlas/pkg/common"
	"github.com/ekristen/atlas/pkg/config"
	"github.com/ekristen/atlas/pkg/envoy"
)

//go:embed templates/*
var templates embed.FS

type Controller struct {
	ctx    context.Context
	config *config.ControllerConfig
	log    *logrus.Entry
	cli    *cli.Context

	apply         apply.Apply
	secrets       core.SecretController
	secretsCache  core.SecretCache
	configmaps    core.ConfigMapController
	services      core.ServiceController
	servicesCache core.ServiceCache

	caPEM    []byte
	caCrt    *x509.Certificate
	caKey    *rsa.PrivateKey
	caSerial string
	caSecret *corev1.Secret

	dnsUpdateLock sync.Mutex
	dnsLastHash   string

	namespace string
}

func Register(
	ctx context.Context,
	config *config.ControllerConfig,
	log *logrus.Entry,
	cli *cli.Context,
	apply apply.Apply,
	secrets core.SecretController,
	configmaps core.ConfigMapController,
	services core.ServiceController,
) (*Controller, error) {
	c := Controller{
		ctx:           ctx,
		config:        config,
		log:           log.WithField("component-type", "controller").WithField("component", cli.String("namespace")),
		cli:           cli,
		apply:         apply,
		secrets:       secrets,
		secretsCache:  secrets.Cache(),
		configmaps:    configmaps,
		services:      services,
		servicesCache: services.Cache(),
		namespace:     cli.String("namespace"),
	}

	c.secrets.OnChange(ctx, common.NAME, c.handleSecretChange)
	c.services.OnChange(ctx, common.NAME, c.handleServiceChange)
	c.services.OnChange(ctx, common.NAME, c.handleServiceChangeforDNS)

	// Index all services that are Atlas Clusters into the PKI index.
	c.servicesCache.AddIndexer("atlasClusters", func(obj *corev1.Service) ([]string, error) {
		labels := obj.GetLabels()
		if _, ok := labels[common.AtlasClusterLabel]; ok {
			return []string{"pki"}, nil
		}
		return []string{}, nil
	})

	// Watch for changes to Secrets that are marked as Atlas PKI, then enqueue all services that are
	// Atlas clusters so that they'll reprocess and update envoy values for bootstrap with new PKI
	relatedresource.Watch(ctx, "atlas", func(namespace, name string, obj runtime.Object) (result []relatedresource.Key, _ error) {
		if obj == nil {
			return result, nil
		}
		if obj.GetObjectKind().GroupVersionKind().Kind != "Secret" {
			return result, nil
		}

		secret := obj.(*corev1.Secret)
		labels := secret.GetLabels()
		_, caOK := labels[common.IsCALabel]
		_, certOK := labels[common.IsCertLabel]
		if !caOK && !certOK {
			return result, nil
		}

		services, err := c.servicesCache.GetByIndex("atlasClusters", "pki")
		if err != nil {
			return nil, err
		}

		for _, service := range services {
			result = append(result, relatedresource.Key{
				Namespace: service.Namespace,
				Name:      service.Name,
			})
		}

		return result, nil
	}, c.services, c.secrets)

	return &c, nil
}

func (c *Controller) createObservabilityValues() error {
	data := struct {
		ClusterID       string
		EnvoyADSAddress string
		EnvoyADSPort    int64
	}{
		ClusterID:       "atlas",
		EnvoyADSAddress: c.config.ADSAddress,
		EnvoyADSPort:    c.config.ADSPort,
	}

	d, err := templates.ReadFile("templates/envoy-atlas.tmpl")
	if err != nil {
		logrus.WithError(err).Error("unable to read in template")
		return err
	}

	tmpl, err := template.New("zone").Funcs(sprig.TxtFuncMap()).Parse(string(d))
	if err != nil {
		logrus.WithError(err).Error("unable to parse template")
		return err
	}

	var buf bytes.Buffer

	if err := tmpl.Execute(&buf, data); err != nil {
		logrus.WithError(err).Error("unable to execute template")
		return err
	}

	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ObservabilityEnvoyValuesSecretName,
			Namespace: c.namespace,
		},
		StringData: map[string]string{
			"values.yaml": string(buf.Bytes()),
		},
	}

	if err := c.apply.WithCacheTypes(c.secrets).WithSetID(common.ObservabilityEnvoyAtlasOwnerID).WithNoDelete().ApplyObjects(s); err != nil {
		logrus.WithError(err).Error("unable to helm values secret for atlas observability cluster")
		return err
	}

	return nil
}

func (c *Controller) Setup() error {
	if err := c.createObservabilityValues(); err != nil {
		return err
	}

	if err := c.configureCA(); err != nil {
		c.log.WithError(err).Error("unable to setup ca")
		return err
	}

	if err := c.setupPKI(); err != nil {
		c.log.WithError(err).Error("unable to setup pki material")
		return err
	}
	return nil
}

func (c *Controller) handleSecretChange(key string, secret *corev1.Secret) (*corev1.Secret, error) {
	if secret == nil {
		return nil, nil
	}

	annotations := secret.GetAnnotations()
	if v, ok := annotations["objectset.rio.cattle.io/id"]; !ok || (ok && v != common.CAOwnerID) {
		return secret, nil
	}

	labels := secret.GetLabels()
	if _, ok := labels[common.IsCALabel]; ok {
		if err := c.configureCA(); err != nil {
			return secret, err
		}
	}

	if _, ok := labels[common.IsCertLabel]; ok {
		if c.caSecret == nil {
			c.secrets.EnqueueAfter(secret.GetNamespace(), secret.GetName(), 5*time.Second)
			return secret, nil
		}
		if err := c.setupPKI(); err != nil {
			return secret, err
		}
	}

	return secret, nil
}

func (c *Controller) configureCA() error {
	log := logrus.WithField("func", "configureCA")

	log.Debug("start")

	revision := 1
	isNew := false
	doGenerate := false

	var currentCASecret *corev1.Secret

	caSecret, err := c.secrets.Get(c.namespace, common.CASecretName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			isNew = true
			doGenerate = true
		} else {
			return err
		}
	}

	if !isNew {
		log.Debug("is not new")

		labels := caSecret.GetLabels()
		annotations := caSecret.GetAnnotations()
		if _, ok := annotations[common.CARotateAnnotation]; ok {
			log.Debug("going to rotate")
			currentCASecret = caSecret.DeepCopy()
			doGenerate = true

			delete(caSecret.Annotations, common.CARotateAnnotation)
			caSecret, err = c.secrets.Update(caSecret)
			if err != nil {
				return err
			}
		}

		caCert, err := decodePEM(caSecret.Data["ca.pem"])
		if err != nil {
			return err
		}

		c.caSerial = caCert.SerialNumber.String()

		if v, ok := labels[common.CARevisionLabel]; ok {
			r, _ := strconv.Atoi(v)
			revision = r + 1
		}
	}

	log.WithField("generate", doGenerate).Debug("to generate or not generate")

	if doGenerate {
		serial, ca, key, err := c.generateCA()
		if err != nil {
			return err
		}

		caSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        common.CASecretName,
				Namespace:   c.namespace,
				Annotations: map[string]string{},
				Labels: map[string]string{
					common.IsCALabel:       "true",
					common.CARevisionLabel: strconv.Itoa(revision),
					common.CASerialLabel:   serial.String(),
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"ca.pem":     ca.Bytes(),
				"ca-key.pem": key.Bytes(),
			},
		}

		if !isNew {
			currentCALabels := currentCASecret.GetLabels()
			serial := currentCALabels[common.CASerialLabel]

			caSecret.Data[fmt.Sprintf("ca-%s.pem", serial)] = currentCASecret.Data["ca.pem"]

			for k, v := range currentCASecret.Data {
				if k == "ca-key.pem" || k == "ca.pem" {
					continue
				}

				caSecret.Data[k] = v
			}
		}

		if err := c.apply.WithCacheTypes(c.secrets).WithSetID(common.CAOwnerID).ApplyObjects(caSecret); err != nil {
			return err
		}

		log.Info("generated/rotated certificate authority")
	}

	if v, ok := caSecret.Data["ca.pem"]; ok {
		block, _ := pem.Decode([]byte(v))
		if block == nil {
			return fmt.Errorf("failed to parse certificate PEM")
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return err
		}

		c.caPEM = v
		c.caCrt = cert
	}
	if v, ok := caSecret.Data["ca-key.pem"]; ok {
		block, _ := pem.Decode([]byte(v))
		if block == nil {
			return fmt.Errorf("failed to parse certificate PEM")
		}

		parsedKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return err
		}

		c.caKey = parsedKey
	}

	c.caSecret = caSecret

	log.Debug("finished")

	return nil
}

func (c *Controller) setupPKI() error {
	doGenerate := false

	ingressTLSSecret, err := c.secrets.Get(c.namespace, common.IngressTLSSecretName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	} else if err != nil && apierrors.IsNotFound(err) {
		doGenerate = true
	} else if err == nil {
		l := ingressTLSSecret.GetLabels()
		if v, ok := l[common.CASignedSerial]; !ok || (ok && v != c.caSerial) {
			doGenerate = true
		}
	}

	mtlsClientSecret, err := c.secrets.Get(c.namespace, common.ClientSecretName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	} else if err != nil && apierrors.IsNotFound(err) {
		doGenerate = true
	} else if err == nil {
		l := mtlsClientSecret.GetLabels()
		if v, ok := l[common.CASignedSerial]; !ok || (ok && v != c.caSerial) {
			doGenerate = true
		}
	}

	mtlsServerSecret, err := c.secrets.Get(c.namespace, common.ServerSecretName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	} else if err != nil && apierrors.IsNotFound(err) {
		doGenerate = true
	} else if err == nil {
		l := mtlsServerSecret.GetLabels()
		if v, ok := l[common.CASignedSerial]; !ok || (ok && v != c.caSerial) {
			doGenerate = true
		}
	}

	ingressSerial, ingressCert, ingressKey, ingressChecksum, err := c.generateCert([]x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, c.config.EnvoyAddress)
	if err != nil {
		return err
	}

	serverSerial, serverCert, serverKey, serverChecksum, err := c.generateCert([]x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, "server.atlas")
	if err != nil {
		return err
	}

	clientSerial, clientCert, clientKey, clientChecksum, err := c.generateCert([]x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, "client.atlas")
	if err != nil {
		return err
	}

	ingressLabels := ingressTLSSecret.GetLabels()
	if v, ok := ingressLabels[common.CAChecksumLabel]; !ok || (ok && v != *ingressChecksum) {
		doGenerate = true
	}

	serverLabels := mtlsServerSecret.GetLabels()
	if v, ok := serverLabels[common.CAChecksumLabel]; !ok || (ok && v != *serverChecksum) {
		doGenerate = true
	}

	clientLabels := mtlsClientSecret.GetLabels()
	if v, ok := clientLabels[common.CAChecksumLabel]; !ok || (ok && v != *clientChecksum) {
		doGenerate = true
	}

	if doGenerate {
		ingressTLSSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.IngressTLSSecretName,
				Namespace: c.namespace,
				Labels: map[string]string{
					common.IsCertLabel:    "true",
					common.CASerialLabel:  fmt.Sprintf("%d", ingressSerial),
					common.CASignedSerial: c.caSerial,
				},
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{
				"ca.crt":  c.caPEM,
				"tls.crt": ingressCert.Bytes(),
				"tls.key": ingressKey.Bytes(),
			},
		}

		mtlsClientSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ClientSecretName,
				Namespace: c.namespace,
				Labels: map[string]string{
					common.IsCertLabel:        "true",
					common.CASerialLabel:      fmt.Sprintf("%d", clientSerial),
					common.CAUsageClientLabel: "true",
					common.CASignedSerial:     c.caSerial,
				},
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{
				"ca.crt":  c.caPEM,
				"tls.crt": clientCert.Bytes(),
				"tls.key": clientKey.Bytes(),
			},
		}

		mtlsServerSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ServerSecretName,
				Namespace: c.namespace,
				Labels: map[string]string{
					common.IsCertLabel:        "true",
					common.CASerialLabel:      fmt.Sprintf("%d", serverSerial),
					common.CAUsageServerLabel: "true",
					common.CASignedSerial:     c.caSerial,
				},
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{
				"ca.crt":  c.caPEM,
				"tls.crt": serverCert.Bytes(),
				"tls.key": serverKey.Bytes(),
			},
		}

		if err := c.apply.WithCacheTypes(c.secrets).WithOwner(c.caSecret).ApplyObjects(ingressTLSSecret, mtlsClientSecret, mtlsServerSecret); err != nil {
			return err
		}
	}

	return nil
}

func (c *Controller) generateCert(extKeyUsage []x509.ExtKeyUsage, commonName string) (*big.Int, *bytes.Buffer, *bytes.Buffer, *string, error) {
	serial := big.NewInt(time.Now().UTC().Unix())

	subject := pkix.Name{
		Organization:       []string{"goatlas.io"},
		OrganizationalUnit: []string{"Atlas"},
		Country:            []string{"US"},
		Province:           []string{"DC"},
		Locality:           []string{"Washington"},
		CommonName:         commonName,
	}

	cert := &x509.Certificate{
		SerialNumber: serial,
		Subject:      subject,
		// IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(10, 0, 0),
		// SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage: extKeyUsage,
		KeyUsage:    x509.KeyUsageDigitalSignature,
	}

	certPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, c.caCrt, &certPrivKey.PublicKey, c.caKey)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	certPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey),
	})

	hash, err := hashstructure.Hash(map[string]interface{}{
		"subject":     subject,
		"extKeyUsage": extKeyUsage,
	}, hashstructure.FormatV2, nil)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	hs := fmt.Sprintf("%d", hash)

	return serial, certPEM, certPrivKeyPEM, &hs, nil
}

func (c *Controller) generateCA() (*big.Int, *bytes.Buffer, *bytes.Buffer, error) {
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UTC().Unix()),
		Subject: pkix.Name{
			Organization:       []string{"goatlas.io"},
			OrganizationalUnit: []string{"Atlas"},
			Country:            []string{"US"},
			Province:           []string{"DC"},
			Locality:           []string{"Washington"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, nil, err
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, nil, nil, err
	}

	caPEM := new(bytes.Buffer)
	if err := pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	}); err != nil {
		return nil, nil, nil, err
	}

	caPrivKeyPEM := new(bytes.Buffer)
	if err := pem.Encode(caPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey),
	}); err != nil {
		return nil, nil, nil, err
	}

	return ca.SerialNumber, caPEM, caPrivKeyPEM, nil
}

func decodePEM(certPEM []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to parse certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}

	return cert, nil
}

func (c *Controller) handleServiceChange(key string, service *corev1.Service) (*corev1.Service, error) {
	if service == nil {
		return nil, nil
	}

	labels := service.GetLabels()
	if _, ok := labels[common.AtlasClusterLabel]; !ok {
		return service, nil
	}

	resolvedSelectors := map[string]string{}
	defaultSelectors := common.EnvoySelectors

	annotations := service.GetAnnotations()
	if v, ok := annotations[common.EnvoySelectorsAnnotation]; ok {
		defaultSelectors = v
	}

	for _, p := range strings.Split(defaultSelectors, ",") {
		s := strings.Split(p, "=")
		resolvedSelectors[s[0]] = s[1]
	}

	replicas := 1
	if v, ok := labels[common.ReplicasLabel]; ok {
		i, err := strconv.Atoi(v)
		if err != nil {
			logrus.WithError(err).Error("unable to convert string to int")
		} else {
			replicas = i
		}
	}

	objs := []runtime.Object{}

	for i := 0; i < replicas; i++ {
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-thanos-sidecar%d", service.Name, i),
				Namespace: service.GetNamespace(),
				Labels: map[string]string{
					common.SidecarLabel: fmt.Sprintf("%d", i),
				},
			},
			Spec: corev1.ServiceSpec{
				ClusterIP: "None",
				Ports:     service.Spec.Ports,
				Type:      corev1.ServiceTypeClusterIP,
				Selector:  resolvedSelectors,
			},
		}

		objs = append(objs, service)
	}

	s, err := c.generateEnvoyValuesSecret(service)
	if err != nil {
		return service, err
	}
	objs = append(objs, s)

	if err := c.apply.WithCacheTypes(c.secrets, c.services).WithSetOwnerReference(false, false).WithOwner(service).ApplyObjects(objs...); err != nil {
		logrus.WithError(err).Error("unable to create thanos-sidecar service")
		return service, err
	}

	return service, nil
}

func (c *Controller) handleServiceChangeforDNS(key string, service *corev1.Service) (*corev1.Service, error) {
	if service == nil {
		return nil, nil
	}

	c.dnsUpdateLock.Lock()
	defer c.dnsUpdateLock.Unlock()

	monitoringNamespace := c.namespace

	requirement, err := labels.NewRequirement(common.SidecarLabel, selection.Exists, []string{})
	if err != nil {
		logrus.WithError(err).Error("unable to build label requirement")
		return service, nil
	}

	selector := labels.NewSelector().Add(*requirement)
	services, err := c.servicesCache.List(monitoringNamespace, selector)
	if err != nil {
		logrus.WithError(err).Error("unable to get list of services")
		return service, nil
	}

	srvrecords := []string{}

	for _, s := range services {
		for _, p := range s.Spec.Ports {
			record := fmt.Sprintf(
				"_%s._%s.sidecars.thanos.atlas. 60 IN SRV 10 100 %s %s.%s.svc.cluster.local.",
				strings.ToLower(p.Name),
				strings.ToLower(string(p.Protocol)),
				&p.TargetPort,
				s.Name,
				monitoringNamespace,
			)

			srvrecords = append(srvrecords, record)
		}
	}

	sort.Strings(srvrecords)

	h := md5.New()
	if _, err := io.WriteString(h, strings.Join(srvrecords, "\n")); err != nil {
		return service, err
	}
	newHash := fmt.Sprintf("%x", h.Sum(nil))

	if newHash == c.dnsLastHash {
		logrus.WithField("last", c.dnsLastHash).WithField("new", newHash).Debug("dns hashes match")
		return service, nil
	}

	c.dnsLastHash = newHash

	data := struct {
		Serial  int64
		Records []string
	}{
		Serial:  time.Now().UTC().Unix(),
		Records: srvrecords,
	}

	d, err := templates.ReadFile("templates/zone.tmpl")
	if err != nil {
		logrus.WithError(err).Error("unable to read template file")
		return service, nil
	}

	tmpl, err := template.New("zone").Funcs(sprig.TxtFuncMap()).Parse(string(d))
	if err != nil {
		logrus.WithError(err).Error("unable to parse template file")
		return service, nil
	}

	var buf bytes.Buffer

	if err := tmpl.Execute(&buf, data); err != nil {
		logrus.WithError(err).Error("unable to execute template")
		return service, nil
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.cli.String("dns-config-map-name"),
			Namespace: c.namespace,
		},
		Data: map[string]string{
			"atlas.zone": string(buf.Bytes()),
		},
	}

	if err := c.apply.WithCacheTypes(c.configmaps).WithSetID(common.DNSOwnerID).ApplyObjects(cm); err != nil {
		logrus.WithError(err).Error("unable to create dns config map for thanos-query service discovery")
		return service, nil
	}

	return service, nil
}

func (c *Controller) generateEnvoyValuesSecret(service *corev1.Service) (*corev1.Secret, error) {
	ca, err := c.secretsCache.Get(c.namespace, common.CASecretName)
	if err != nil {
		return nil, err
	}

	server, err := c.secretsCache.Get(c.namespace, common.ServerSecretName)
	if err != nil {
		return nil, err
	}

	client, err := c.secretsCache.Get(c.namespace, common.ClientSecretName)
	if err != nil {
		return nil, err
	}

	actualAMServices := []*corev1.Service{}
	amServices, err := c.services.List(c.namespace, v1.ListOptions{
		LabelSelector: c.cli.String("alertmanager-selector"),
	})
	if err != nil {
		return nil, err
	}
	for _, service := range amServices.Items {
		if _, ok := service.Spec.Selector["statefulset.kubernetes.io/pod-name"]; ok {
			actualAMServices = append(actualAMServices, &service)
		}
	}

	data := struct {
		CA                string
		ServerCert        string
		ServerKey         string
		ClientCert        string
		ClientKey         string
		ClusterID         string
		EnvoyADSAddress   string
		EnvoyADSPort      int64
		AlertmanagerCount int
	}{
		CA:                string(envoy.CombineCAs(ca)),
		ServerCert:        string(server.Data["tls.crt"]),
		ServerKey:         string(server.Data["tls.key"]),
		ClientCert:        string(client.Data["tls.crt"]),
		ClientKey:         string(client.Data["tls.key"]),
		ClusterID:         service.Name,
		EnvoyADSAddress:   c.config.ADSAddress,
		EnvoyADSPort:      c.config.ADSPort,
		AlertmanagerCount: len(actualAMServices),
	}

	d, err := templates.ReadFile("templates/envoy-downstream.tmpl")
	if err != nil {
		logrus.WithError(err).Error("unable to read in template")
		return nil, err
	}

	tmpl, err := template.New("zone").Funcs(sprig.TxtFuncMap()).Parse(string(d))
	if err != nil {
		logrus.WithError(err).Error("unable to parse template")
		return nil, err
	}

	var buf bytes.Buffer

	if err := tmpl.Execute(&buf, data); err != nil {
		logrus.WithError(err).Error("unable to execute template")
		return nil, err
	}

	secretName := fmt.Sprintf("%s-envoy-values", service.Name)
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: c.namespace,
		},
		StringData: map[string]string{
			"values.yaml": string(buf.Bytes()),
		},
	}

	return s, nil
}
