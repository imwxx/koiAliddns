package main

import (
	"errors"
	"flag"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/auth/credentials"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/alidns"
	"github.com/digineo/go-uci"
	"github.com/sirupsen/logrus"
)

var logger *logrus.Logger

type Flags struct {
	file string
	show string
}

var file = "/etc/config"

var show = `
create config like this
k@k-ThinkPad-P15-Gen-1:~$ cat /etc/config/koiAliddns
config cron
    #At every 20th minute
    option time "0 */20 * * * *"

config auth
	option ak "xxxxxxxxxxxx"
	option sk "xxxxxxxxxxxxxxxxxx"

config host
	option rr "abc"
	option type "A"
	option ttl "600"
	option priority "1"
	option line "default"
	option domain "wuxuxing.com"
`

type Hosts struct {
	RR       string
	TYPE     string
	TTL      int
	PRIORITY int
	LINE     string
	DOMAIN   string
	RECORDID string
	VALUE    string
}

type AKSK struct {
	AK string
	SK string
}

type NewUCI struct {
	WANIP string
	UCI   uci.Tree
}

func parseFlags() *Flags {
	argsflag := new(Flags)
	flag.StringVar(&argsflag.file, "f", file, "config dir, default /etc/config")
	flag.StringVar(&argsflag.show, "s", show, "show config demo")
	flag.Parse()
	return argsflag
}

func (i *NewUCI) GetAKSK() (AKSK, error) {
	var aksk AKSK
	if vaule, ok := i.UCI.Get("koiAliddns", "@auth[0]", "ak"); ok {
		aksk.AK = vaule[0]
	} else {
		return aksk, errors.New("auth section null")
	}
	if vaule, ok := i.UCI.Get("koiAliddns", "@auth[0]", "sk"); ok {
		aksk.SK = vaule[0]
	} else {
		return aksk, errors.New("auth section null")
	}
	return aksk, nil
}

func (i *NewUCI) GetHosts() (map[string][]Hosts, error) {
	hosts := make(map[string][]Hosts)

	sections, ok := i.UCI.GetSections("koiAliddns", "host")
	if !ok {
		return hosts, errors.New("get sections error")
	}
	for _, section := range sections {
		var host Hosts
		if val, ok := i.UCI.Get("koiAliddns", section, "rr"); ok {
			host.RR = val[0]
		} else {
			return hosts, errors.New("get pr record error")
		}
		if val, ok := i.UCI.Get("koiAliddns", section, "type"); ok {
			host.TYPE = val[0]
		} else {
			return hosts, errors.New("get pr record error")
		}
		if val, ok := i.UCI.Get("koiAliddns", section, "ttl"); ok {
			host.TTL, _ = strconv.Atoi(val[0])
		} else {
			return hosts, errors.New("get ttl record error")
		}
		if val, ok := i.UCI.Get("koiAliddns", section, "priority"); ok {
			host.PRIORITY, _ = strconv.Atoi(val[0])
		} else {
			return hosts, errors.New("get priority record error")
		}
		if val, ok := i.UCI.Get("koiAliddns", section, "line"); ok {
			host.LINE = val[0]
		} else {
			return hosts, errors.New("get line record error")
		}
		if val, ok := i.UCI.Get("koiAliddns", section, "domain"); ok {
			host.DOMAIN = val[0]
		} else {
			return hosts, errors.New("get domain record error")
		}
		if _, ok := hosts[host.DOMAIN]; ok {
			hosts[host.DOMAIN] = append(hosts[host.DOMAIN], host)
		} else {
			hosts[host.DOMAIN] = []Hosts{host}
		}
	}
	return hosts, nil
}

func (i *NewUCI) HostsHandler() ([]Hosts, []Hosts, error) {
	hostRecords := make(map[string]map[string]alidns.Record)
	data, err := i.GetHosts()
	var uHosts, aHosts []Hosts
	if err != nil {
		return uHosts, aHosts, err
	}

	aksk, err := i.GetAKSK()
	if err != nil {
		return uHosts, aHosts, err

	}

	aliClient := AKSK{
		AK: aksk.AK,
		SK: aksk.SK,
	}

	for domain, _ := range data {
		res, err := aliClient.ListAllDomainRecords(domain)
		if err != nil {
			return uHosts, aHosts, err
		}
		hostRecords[domain] = res[domain]
	}

	for host, infos := range data {
		for _, info := range infos {
			if record, ok := hostRecords[host][info.RR]; ok {
				if record.Value != i.WANIP {
					info.RECORDID = record.RecordId
					info.VALUE = i.WANIP
					uHosts = append(uHosts, info)
				}
			} else {
				info.VALUE = i.WANIP
				aHosts = append(aHosts, info)
			}
		}
	}
	return uHosts, aHosts, nil
}

func (i *AKSK) DescribeDomainRecordsRequest(domain string, pageNum, pageSize int) (*alidns.DescribeDomainRecordsResponse, error) {
	config := sdk.NewConfig()

	credential := credentials.NewAccessKeyCredential(i.AK, i.SK)

	client, err := alidns.NewClientWithOptions("cn-hangzhou", config, credential)
	if err != nil {
		panic(err)
	}

	request := alidns.CreateDescribeDomainRecordsRequest()

	request.Scheme = "https"

	request.PageNumber = requests.NewInteger(pageNum)
	request.PageSize = requests.NewInteger(pageSize)
	request.DomainName = domain

	response, err := client.DescribeDomainRecords(request)
	if err != nil {
		return response, err
	}
	return response, nil
}

func (i *AKSK) ListAllDomainRecords(domain string) (map[string]map[string]alidns.Record, error) {
	data := make(map[string]map[string]alidns.Record)
	pageNum := 1
	pageSize := 500
	res, err := i.DescribeDomainRecordsRequest(domain, pageNum, pageSize)
	if err != nil {
		return nil, err
	}
	total := res.TotalCount
	for {
		if int64(pageNum*pageSize) >= total {
			break
		} else {
			pageNum += 1
			response, err := i.DescribeDomainRecordsRequest(domain, pageNum, pageSize)
			if err != nil {
				return nil, err
			} else {
				res.DomainRecords.Record = append(res.DomainRecords.Record, response.DomainRecords.Record...)
			}
		}
	}
	data[domain] = map[string]alidns.Record{}
	for _, record := range res.DomainRecords.Record {
		data[domain][record.RR] = record
	}
	return data, nil

}

func (i *AKSK) UpdateDNS(hosts []Hosts) (err error) {
	client, err := alidns.NewClientWithAccessKey("cn-hangzhou", i.AK, i.SK)

	for _, host := range hosts {
		request := alidns.CreateUpdateDomainRecordRequest()
		request.Scheme = "https"

		request.RecordId = host.RECORDID
		request.RR = host.RR
		request.Type = host.TYPE
		request.Value = host.VALUE
		request.Lang = "en"
		request.UserClientIp = host.VALUE
		request.TTL = requests.NewInteger(host.TTL)
		request.Priority = requests.NewInteger(host.PRIORITY)
		request.Line = host.LINE

		response, err := client.UpdateDomainRecord(request)
		if err != nil {
			logger.Error(response)
			return err
		}
	}

	return nil
}

func (i *AKSK) AddDNS(hosts []Hosts) (err error) {
	client, err := alidns.NewClientWithAccessKey("cn-hangzhou", i.AK, i.SK)

	for _, host := range hosts {
		request := alidns.CreateAddDomainRecordRequest()
		request.Scheme = "https"
		request.DomainName = host.DOMAIN
		request.RR = host.RR
		request.Type = host.TYPE
		request.Value = host.VALUE
		request.TTL = requests.NewInteger(host.TTL)
		if host.PRIORITY == 0 {
			request.Priority = requests.NewInteger(host.PRIORITY)
		} else {
			request.Priority = requests.NewInteger(1)

		}
		request.Priority = requests.NewInteger(1)
		if host.LINE == "" {
			request.Line = "default"
		} else {
			request.Line = host.LINE
		}

		response, err := client.AddDomainRecord(request)
		if err != nil {
			logger.Error(response)
			return err
		}
	}

	return nil
}

func GetwanIP() (string, error) {
	var wanIp string
	client := &http.Client{}
	req, _ := http.NewRequest("GET", "http://v4.ident.me", nil)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	re, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		logger.Warn("Invalid request: ", string(re))
		return wanIp, errors.New(string(re))
	} else {
		wanIp = string(re)
		wanIp = strings.Replace(wanIp, "", "", -1)
		wanIp = strings.Replace(wanIp, "\n", "", -1)
	}
	return wanIp, err
}

func run(uci NewUCI) {
	// loc, _ := time.LoadLocation("Asia/ShangHai")
	logger.Infof(`%s: koifq alidns check start`, time.Now().Format("2006-01-02 15:04:05 +0800"))

	aksk, err := uci.GetAKSK()
	if err != nil {
		logger.Error(err)
		return
	}
	ali := AKSK{
		AK: aksk.AK,
		SK: aksk.SK,
	}
	updates, adds, err := uci.HostsHandler()
	if err != nil {
		logger.Error(err)
		return
	}

	switch {
	case len(updates) > 0:
		logger.Infof(`%s: koifq alidns update, affected: %d`, time.Now().Format("2006-01-02 15:04:05 +0800"), len(updates))
		ali.UpdateDNS(updates)
	case len(adds) > 0:
		logger.Infof(`%s: koifq alidns add, affected: %d`, time.Now().Format("2006-01-02 15:04:05 +0800"), len(updates))
		ali.AddDNS(adds)
	default:
		logger.Infof(`%s: koifq alidns check end, no new records`, time.Now().Format("2006-01-02 15:04:05 +0800"))
	}
}

func init() {
	logger = logrus.New()
	logger.Formatter = &logrus.JSONFormatter{}
	logger.Level = logrus.InfoLevel
	logger.Out = os.Stderr
}

func main() {
	argsflag := parseFlags()

	file := argsflag.file
	ip, err := GetwanIP()
	if err != nil {
		logger.Error(err)
		return
	}
	uci := NewUCI{
		WANIP: ip,
		UCI:   uci.NewTree(file),
	}

	for {
		go func(uci NewUCI) {
			run(uci)
		}(uci)
		time.Sleep(5 * 60 * time.Second)
	}
}
