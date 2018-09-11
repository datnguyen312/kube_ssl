package main

import (
	// "flag"
    "io"
    "log"
	// "fmt"
	"time"
	"net/http"
	"io/ioutil"
	"encoding/base64"
	"github.com/gin-gonic/gin"
	rest "k8s.io/client-go/rest"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1beta1 "k8s.io/api/certificates/v1beta1"
	certificatesv1beta1 "k8s.io/client-go/kubernetes/typed/certificates/v1beta1"
)

var (
    Trace   *log.Logger
    Info    *log.Logger
    Warning *log.Logger
    Error   *log.Logger
)

func Init(
    traceHandle io.Writer,
    infoHandle io.Writer,
    warningHandle io.Writer,
    errorHandle io.Writer) {

    Trace = log.New(traceHandle,
        "TRACE: ",
        log.Ldate|log.Ltime|log.Lshortfile)

    Info = log.New(infoHandle,
        "INFO: ",
        log.Ldate|log.Ltime|log.Lshortfile)

    Warning = log.New(warningHandle,
        "WARNING: ",
        log.Ldate|log.Ltime|log.Lshortfile)

    Error = log.New(errorHandle,
        "ERROR: ",
        log.Ldate|log.Ltime|log.Lshortfile)
}


func OutClusterConfig() *rest.Config {
	certData, _ := ioutil.ReadFile("/etc/kubernetes/pki/apiserver-kubelet-client.crt")

	keyData, _ := ioutil.ReadFile("/etc/kubernetes/pki/apiserver-kubelet-client.key")

	config := &rest.Config{
		Host:    "https://127.0.0.1:6443",
		APIPath: "/api",
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
			CertFile: "/etc/kubernetes/pki/apiserver-kubelet-client.crt",
			KeyFile:  "/etc/kubernetes/pki/apiserver-kubelet-client.key",
			CertData: certData,
			KeyData:  keyData,
		},
	}

	return config
}

type clientQuery struct {
	Nodename string `json:"nodename"`
	Nodeip 	 string `json:"nodeip"`
	CsrData  string `json:"csrdata"`
}

var Config *rest.Config

// func csrApprover() {
// 	certConfig, err := certificatesv1beta1.NewForConfig(Config)
// 	if err != nil { panic(err) }	

// 	eventChan, errEvent := certConfig.CertificateSigningRequests().Watch(v1.ListOptions{})
// 	if errEvent != nil { panic(errEvent) }

// 	ch := eventChan.ResultChan()
// 	for {
// 		msg := <-ch
// 		certRequest := msg.Object.(*v1beta1.CertificateSigningRequest)
// 		fmt.Println(certRequest.ObjectMeta.Name)
// 		if certRequest.Status.Conditions == nil {
// 			certRequest.Status.Conditions = append(certRequest.Status.Conditions, v1beta1.CertificateSigningRequestCondition{
// 				Type: v1beta1.CertificateApproved,
// 			})
// 			_, err := certConfig.CertificateSigningRequests().UpdateApproval(certRequest)
// 			if err != nil { panic(err) }
// 		}
// 	}
// }

func csrApprover(r clientQuery) error {
	// Just sleep 1 sec to make sure the object has been sync
	time.Sleep(time.Duration(1) * time.Second)
	certConfig, err := certificatesv1beta1.NewForConfig(Config)
	if err != nil { panic(err) }	

	certRequest, errRequest := certConfig.CertificateSigningRequests().Get(r.Nodeip, v1.GetOptions{})
	certRequest.Status.Conditions = append(certRequest.Status.Conditions, v1beta1.CertificateSigningRequestCondition{
		Type: v1beta1.CertificateApproved, // Approved cert
	})

	if errRequest != nil { panic(errRequest) }
	_, err = certConfig.CertificateSigningRequests().UpdateApproval(certRequest)
	if err != nil { 
		Error.Println(err) 
		return err
	}
	return err
}

func csrRequester(r clientQuery) error {
	var decoded []byte
	certConfig, err := certificatesv1beta1.NewForConfig(Config)
	if err != nil { panic(err) }	

	decoded, err = base64.StdEncoding.DecodeString(r.CsrData)
	if err != nil {
		Error.Println(err)
		return err
	} else {
		var certRequest = &v1beta1.CertificateSigningRequest{
			ObjectMeta: v1.ObjectMeta{
				Name: r.Nodeip,
			},
			Spec: v1beta1.CertificateSigningRequestSpec{
				Usages: []v1beta1.KeyUsage{
					v1beta1.UsageServerAuth,
					v1beta1.UsageClientAuth,
				},
				Request: decoded,
			},
		}

		_, err = certConfig.CertificateSigningRequests().Create(certRequest)
		if err != nil { 
			Error.Println(err) 
			return err
		}	
	}
	return err
}

func crtDownload(r clientQuery) string {
	// Just sleep 1 sec to make sure the object has been sync
	time.Sleep(time.Duration(1) * time.Second)
	certConfig, err := certificatesv1beta1.NewForConfig(Config)
	if err != nil { panic(err) }
	certRequest, errRequest := certConfig.CertificateSigningRequests().Get(r.Nodeip, v1.GetOptions{})
	if errRequest != nil { panic(errRequest) }

	return string(certRequest.Status.Certificate)
}

func request(c *gin.Context) {
	var r clientQuery
	c.BindJSON(&r)
	if r.Nodename != "" && r.Nodeip != "" && r.CsrData != ""{
		if err := csrRequester(r); err != nil {
			c.String(500, err.Error())
		}
		if err := csrApprover(r); err != nil {
			c.String(500, err.Error())	
		}
		certCRT := crtDownload(r)
		c.String(200, certCRT)
	} else {
		c.String(404, "Missing nodename or nodeip or csrdata")		
	}
}

func main() {
	Config = OutClusterConfig()
	// go csrApprover()

	r := gin.Default()
	r.POST("v1/request", request)
	s := &http.Server{
		Addr:           ":9999",
		Handler:        r,
		ReadTimeout:    120 * time.Second,
		WriteTimeout:   120 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	s.ListenAndServe()
}
