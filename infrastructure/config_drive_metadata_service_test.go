package infrastructure_test

import (
	"encoding/json"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	fakeinf "github.com/cloudfoundry/bosh-agent/infrastructure/fakes"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	fakeplatform "github.com/cloudfoundry/bosh-agent/platform/fakes"

	. "github.com/cloudfoundry/bosh-agent/infrastructure"
)

var _ = Describe("ConfigDriveMetadataService", func() {
	var (
		metadataService MetadataService
		resolver        *fakeinf.FakeDNSResolver
		platform        *fakeplatform.FakePlatform
	)

	updateMetadata := func(metadataContents MetadataContentsType) {
		metadataJSON, err := json.Marshal(metadataContents)
		Expect(err).ToNot(HaveOccurred())
		platform.SetGetFilesContentsFromDisk("fake-metadata-path", metadataJSON, nil)

		err = metadataService.Load()
		Expect(err).ToNot(HaveOccurred())
	}

	updateUserdata := func(userdataContents string) {
		platform.SetGetFilesContentsFromDisk("fake-userdata-path", []byte(userdataContents), nil)

		err := metadataService.Load()
		Expect(err).ToNot(HaveOccurred())
	}

	BeforeEach(func() {
		resolver = &fakeinf.FakeDNSResolver{}
		platform = fakeplatform.NewFakePlatform()
		logger := boshlog.NewLogger(boshlog.LevelNone)
		diskPaths := []string{
			"fake-disk-path-1",
			"fake-disk-path-2",
		}
		metadataService = NewConfigDriveMetadataService(
			resolver,
			platform,
			diskPaths,
			"fake-metadata-path",
			"fake-userdata-path",
			logger,
		)

		userdataContents := fmt.Sprintf(`{"server":{"name":"fake-server-name"},"registry":{"endpoint":"fake-registry-endpoint"}}`)
		platform.SetGetFilesContentsFromDisk("fake-userdata-path", []byte(userdataContents), nil)

		metadata := MetadataContentsType{
			PublicKeys: map[string]PublicKeyType{
				"0": PublicKeyType{
					"openssh-key": "fake-openssh-key",
				},
			},
			InstanceID: "fake-instance-id",
		}
		updateMetadata(metadata)
	})

	Describe("Load", func() {
		It("returns an error if it fails to read meta-data.json from disk", func() {
			platform.SetGetFilesContentsFromDisk("fake-metadata-path", []byte{}, errors.New("fake-read-disk-error"))
			err := metadataService.Load()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-read-disk-error"))
		})

		It("tries to load meta-data.json from potential disk locations", func() {
			platform.SetGetFilesContentsFromDisk("fake-metadata-path", []byte{}, errors.New("fake-read-disk-error"))
			err := metadataService.Load()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-read-disk-error"))

			Expect(platform.GetFileContentsFromDiskDiskPaths).To(ContainElement("fake-disk-path-1"))
			Expect(platform.GetFileContentsFromDiskDiskPaths).To(ContainElement("fake-disk-path-2"))
		})

		It("returns an error if it fails to parse meta-data.json contents", func() {
			platform.SetGetFilesContentsFromDisk("fake-metadata-path", []byte("broken"), nil)
			err := metadataService.Load()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Parsing config drive metadata from meta_data.json"))
		})

		It("returns an error if it fails to read user_data from disk", func() {
			platform.SetGetFilesContentsFromDisk("fake-userdata-path", []byte{}, errors.New("fake-read-disk-error"))
			err := metadataService.Load()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-read-disk-error"))
		})

		It("returns an error if it fails to parse user_data contents", func() {
			platform.SetGetFilesContentsFromDisk("fake-userdata-path", []byte("broken"), nil)
			err := metadataService.Load()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Parsing config drive metadata from user_data"))
		})
	})

	Describe("GetPublicKey", func() {
		It("returns public key", func() {
			value, err := metadataService.GetPublicKey()
			Expect(err).ToNot(HaveOccurred())
			Expect(value).To(Equal("fake-openssh-key"))
		})

		It("returns an error if it fails to get ssh key", func() {
			updateMetadata(MetadataContentsType{})

			value, err := metadataService.GetPublicKey()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to load openssh-key from config drive metadata service"))

			Expect(value).To(Equal(""))
		})
	})

	Describe("GetInstanceID", func() {
		It("returns instance id", func() {
			value, err := metadataService.GetInstanceID()
			Expect(err).ToNot(HaveOccurred())
			Expect(value).To(Equal("fake-instance-id"))
		})

		It("returns an error if it fails to get instance id", func() {
			updateMetadata(MetadataContentsType{})

			value, err := metadataService.GetInstanceID()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to load instance-id from config drive metadata service"))

			Expect(value).To(Equal(""))
		})
	})

	Describe("GetServerName", func() {
		It("returns server name", func() {
			value, err := metadataService.GetServerName()
			Expect(err).ToNot(HaveOccurred())
			Expect(value).To(Equal("fake-server-name"))
		})

		It("returns an error if it fails to get server name", func() {
			updateUserdata("{}")

			value, err := metadataService.GetServerName()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to load server name from config drive metadata service"))

			Expect(value).To(Equal(""))
		})
	})

	Describe("GetRegistryEndpoint", func() {
		It("returns an error if it fails to get registry endpoint", func() {
			updateUserdata("{}")

			value, err := metadataService.GetRegistryEndpoint()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Failed to load registry endpoint from config drive metadata service"))

			Expect(value).To(Equal(""))
		})

		Context("when user_data does not contain a dns server", func() {
			It("returns registry endpoint", func() {
				value, err := metadataService.GetRegistryEndpoint()
				Expect(err).ToNot(HaveOccurred())
				Expect(value).To(Equal("fake-registry-endpoint"))
			})
		})

		Context("when user_data contains a dns server", func() {
			BeforeEach(func() {
				userdataContents := fmt.Sprintf(
					`{"server":{"name":"%s"},"registry":{"endpoint":"%s"},"dns":{"nameserver":["%s"]}}`,
					"fake-server-name",
					"http://fake-registry.com",
					"fake-dns-server-ip",
				)
				updateUserdata(userdataContents)
			})

			Context("when registry endpoint is successfully resolved", func() {
				BeforeEach(func() {
					resolver.RegisterRecord(fakeinf.FakeDNSRecord{
						DNSServers: []string{"fake-dns-server-ip"},
						Host:       "http://fake-registry.com",
						IP:         "http://fake-registry-ip",
					})
				})

				It("returns the successfully resolved registry endpoint", func() {
					endpoint, err := metadataService.GetRegistryEndpoint()
					Expect(err).ToNot(HaveOccurred())
					Expect(endpoint).To(Equal("http://fake-registry-ip"))
				})
			})

			Context("when registry endpoint is not successfully resolved", func() {
				BeforeEach(func() {
					resolver.LookupHostErr = errors.New("fake-lookup-host-err")
				})

				It("returns error because it failed to resolve registry endpoint", func() {
					endpoint, err := metadataService.GetRegistryEndpoint()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-lookup-host-err"))
					Expect(endpoint).To(BeEmpty())
				})
			})
		})
	})
})
