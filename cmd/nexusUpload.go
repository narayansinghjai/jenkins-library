package cmd

import (
	"encoding/hex"
	"encoding/json"
	"hash"
	"io"
	"net/http"
	"os"
	"strings"

	"crypto/md5"
	"crypto/sha1"

	piperHttp "github.com/SAP/jenkins-library/pkg/http"
	"github.com/SAP/jenkins-library/pkg/log"
	"github.com/SAP/jenkins-library/pkg/telemetry"
	piperUtils "github.com/SAP/jenkins-library/pkg/piperUtils"
)

type artifactDescription struct {
	ID         string `json:"artifactId"`
	Classifier string `json:"classifier"`
	Type       string `json:"type"`
	File       string `json:"file"`
}

func nexusUpload(config nexusUploadOptions, telemetryData *telemetry.CustomData) error {
	mtaYaml, err := piperUtils.FileExists("mta.yaml")
	if mtaYaml {
		upload("foo.mtar")
		upload("mta.yaml")
	}

	

	var artifacts []artifactDescription
	err := json.Unmarshal([]byte(config.Artifacts), &artifacts)
	if err != nil {
		log.Entry().WithError(err).Fatal("Failed to convert artifact JSON '", config.Artifacts, "'")
	}

	client := piperHttp.Client{}
	clientOptions := piperHttp.ClientOptions{Username: config.User, Password: config.Password, Logger: log.Entry().WithField("package", "github.com/SAP/jenkins-library/pkg/http")}
	client.SetOptions(clientOptions)

	groupPath := strings.ReplaceAll(config.GroupID, ".", "/")

	for _, artifact := range artifacts {
		url := config.Url
		switch config.NexusVersion {
		case "nexus2":
			url += "/content/repositories/"
		case "nexus3":
			url += "/repository/"
		default:
			log.Entry().WithError(err).Fatal("Unsupported Nexus version ", config.NexusVersion)
		}

		artifactName := artifact.ID + "-" + config.Version
		if len(artifact.Classifier) > 0 {
			artifactName += "-" + artifact.Classifier
		}
		artifactName += "." + artifact.Type

		url += config.Repository + "/" + groupPath + "/" + artifact.ID + "/" + config.Version + "/" + artifactName
		url = "http://" + strings.ReplaceAll(url, "//", "/")
		log.Entry().Info("Trying to upload ", artifact.File, " to ", url)

		uploadHash(&client, artifact.File, url+".md5", md5.New(), 16)
		uploadHash(&client, artifact.File, url+".sha1", sha1.New(), 20)

		var file *os.File
		file, err = os.Open(artifact.File)
		if err != nil {
			log.Entry().WithError(err).Fatal("Failed to open artifact file ", artifact.File)
		}

		defer file.Close()

		_, err = uploadToNexus(&client, file, url)
		if err != nil {
			log.Entry().WithError(err).Fatal("Failed to upload artifact ", artifact.File)
		}
	}

	return err
}

func uploadToNexus(client *piperHttp.Client, stream io.Reader, url string) (*http.Response, error) {
	response, err := client.SendRequest(http.MethodPut, url, stream, nil, nil)
	if err != nil {
		// if response != nil && response.StatusCode == 400 {
		// 	log.Entry().Info("Artifact already exits, deleting and retrying...\n")
		// 	response, err = client.SendRequest(http.MethodDelete, url, nil, nil, nil)
		// 	if err != nil {
		// 		log.Entry().Info("Failed to delete artifact\n", err)
		// 		return nil, err
		// 	}
		// 	response, err = client.SendRequest(http.MethodPut, url, stream, nil, nil)
		// }
		// if err != nil {
		// 		log.Entry().Info("Failed to upload artifact\n", err)
		//		return nil, err
		// }
	} else {
		log.Entry().Info("Uploaded '"+url+"', response: ", response.StatusCode)
	}
	return response, err
}

func uploadHash(client *piperHttp.Client, filePath, url string, hash hash.Hash, length int) (*http.Response, error) {
	hashReader, err := generateHashReader(filePath, hash, length)
	if err != nil {
		log.Entry().WithError(err).Fatal("Failed to generate hash")
	}
	var response *http.Response
	response, err = uploadToNexus(client, hashReader, url)
	if err != nil {
		log.Entry().WithError(err).Fatal("Failed to upload hash")
	}
	return response, nil
}

func generateHashReader(filePath string, hash hash.Hash, length int) (io.Reader, error) {
	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	// Read file and feed the hash
	if _, err := io.Copy(hash, file); err != nil {
		return nil, err
	}

	// Get the requested number of bytes from the hash
	hashInBytes := hash.Sum(nil)[:length]

	// Convert the bytes to a string
	hexString := hex.EncodeToString(hashInBytes)

	// Finally create an io.Reader wrapping the string
	return strings.NewReader(hexString), nil
}
