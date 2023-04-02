package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/auth/credentials"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/alidns"
	"github.com/digineo/go-uci"
	"github.com/jasonlvhit/gocron"
	"github.com/sirupsen/logrus"
)

type Flags struct {
	file string
	show string
}

var file = "/etc/config"

var show = `
config koiAliddns
	option cron 300
	option enabled '1'
	# 指定被获取IP的网卡地址，非必须；没配置的话，走http://myip.ipip.net/json
	# option eth "enp0s31f6"
	option ipv46 "ipv4"

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

config host
	option rr "efg"
	option type "A"
	option ttl "600"
	option priority "1"
	option line "default"
	option domain "wuxuxing.com"
`

type MyipipipNet struct {
	Ret  string `json:"ret"`
	Data struct {
		Ip       string   `json:"ip"`
		Location []string `json:"location"`
	} `json:"data"`
}

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

type KoiAliDdns struct {
	Cron    int
	Enabled bool
	Eth     string
	Myipapi string
	Ipv46   string
}

type AKSK struct {
	AK string
	SK string
}

type NewUCI struct {
	UCI uci.Tree
}

var (
	stdOut *logrus.Logger
	stdErr *logrus.Logger
	kuci   NewUCI
)

func parseFlags() *Flags {
	argsflag := new(Flags)
	flag.StringVar(&argsflag.file, "f", file, "config dir, default /etc/config")
	flag.StringVar(&argsflag.show, "c", "0", "create sample config demo to /etc/config/koiAliddns, use value 1")
	flag.Parse()
	return argsflag
}

func (i *NewUCI) GetKoiAliDdns() (KoiAliDdns, error) {
	var data KoiAliDdns
	var enabled bool

	if value, ok := i.UCI.Get("koiAliddns", "@koiAliddns[0]", "cron"); ok {
		if t, err := strconv.Atoi(value[0]); err == nil {
			if t < 60 {
				return data, errors.New(fmt.Sprintf(`option cron %s, setting less than 60 second`, value[0]))
			} else {
				data.Cron = t
			}
		} else {
			data.Cron = 300
		}
	}

	if value, ok := i.UCI.Get("koiAliddns", "@koiAliddns[0]", "enabled"); ok {
		e, _ := strconv.Atoi(value[0])
		if e == 1 {
			enabled = true
		} else {
			enabled = false
		}
	} else {
		enabled = false
	}
	data.Enabled = enabled

	if value, ok := i.UCI.Get("koiAliddns", "@koiAliddns[0]", "eth"); ok {
		if len(value) > 0 {
			data.Eth = value[0]
		} else {
			data.Myipapi = "http://myip.ipip.net/json"
		}
	} else {
		data.Myipapi = "http://myip.ipip.net/json"
	}

	if value, ok := i.UCI.Get("koiAliddns", "@koiAliddns[0]", "ipv46"); ok {
		data.Ipv46 = value[0]
		if len(value) > 0 {
			data.Ipv46 = value[0]
		} else {
			data.Ipv46 = "ipv4"
		}
	} else {
		data.Ipv46 = "ipv4"
	}

	return data, nil
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

func (i *NewUCI) GetwanIP(koialiddns KoiAliDdns) (map[string]string, error) {
	wanIp := map[string]string{
		"ipv4": "",
		"ipv6": "",
	}

	if koialiddns.Eth != "" {
		ifaces, _ := net.Interfaces()
		for _, i := range ifaces {
			if i.Name == koialiddns.Eth {
				addrs, _ := i.Addrs()
				for _, addr := range addrs {
					var ip net.IP
					switch v := addr.(type) {
					case *net.IPNet:
						ip = v.IP
					case *net.IPAddr:
						ip = v.IP
					}

					v4 := strings.Split(ip.String(), ".")
					if len(v4) > 1 {
						wanIp["ipv4"] = ip.String()
					}

					v6 := strings.Split(ip.String(), ":")
					if len(v6) > 1 {
						wanIp["ipv6"] = ip.String()
					}

				}
			}
		}
	} else {
		client := &http.Client{}
		req, _ := http.NewRequest("GET", koialiddns.Myipapi, nil)

		var myip string
		resp, err := client.Do(req)
		if err != nil {
			return wanIp, err
		}
		re, _ := ioutil.ReadAll(resp.Body)

		if resp.StatusCode != 200 {
			stdErr.Warn("Invalid request: ", string(re))
			return wanIp, errors.New(string(re))
		} else {
			var b MyipipipNet
			if err := json.Unmarshal(re, &b); err != nil {
				return wanIp, err
			}
			myip = b.Data.Ip
		}

		v4 := strings.Split(string(myip), ".")
		if len(v4) > 0 {
			wanIp["ipv4"] = string(myip)
		}

		v6 := strings.Split(string(myip), ":")
		if len(v6) > 0 {
			wanIp["ipv6"] = string(myip)
		}
	}

	return wanIp, nil
}
func (i *NewUCI) HostsHandler() ([]Hosts, []Hosts, error) {
	hostRecords := make(map[string]map[string]alidns.Record)
	data, err := i.GetHosts()
	var uHosts, aHosts []Hosts
	if err != nil {
		return uHosts, aHosts, err
	}

	koialiddns, err := i.GetKoiAliDdns()
	if err != nil {
		return uHosts, aHosts, err
	}

	wanIp, err := i.GetwanIP(koialiddns)
	if err != nil {
		return nil, nil, err
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

	var wanip string

	if v, ok := wanIp[koialiddns.Ipv46]; ok {
		wanip = v
	}
	for host, infos := range data {
		for _, info := range infos {
			if record, ok := hostRecords[host][info.RR]; ok {
				if record.Value != wanip {
					info.RECORDID = record.RecordId
					info.VALUE = wanip
					uHosts = append(uHosts, info)
				}
			} else {
				info.VALUE = wanip
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
			stdErr.Error(response)
			return err
		}
		stdOut.Infof(`koifq alidns updated, RR: %s, Domain: %s, Type: %s, Value:%s, TTL: %s, Prioriy: %s, Line: %s`, request.RR, host.DOMAIN, request.Type, request.Value, request.TTL, request.Priority, request.Line)

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
			stdErr.Error(response)
			return err
		}
		stdOut.Infof(`koifq alidns added, RR: %s, Domain: %s, Type: %s, Value:%s, TTL: %s, Prioriy: %s, Line: %s`, request.RR, request.DomainName, request.Type, request.Value, request.TTL, request.Priority, request.Line)
	}

	return nil
}

func init() {
	stdErr = logrus.New()
	stdErr.Formatter = &logrus.JSONFormatter{}
	stdErr.Level = logrus.InfoLevel
	stdErr.Out = os.Stderr

	stdOut = logrus.New()
	stdOut.Formatter = &logrus.JSONFormatter{}
	stdOut.Level = logrus.InfoLevel
	stdOut.Out = os.Stdout

}

func initconf(argsflag *Flags) {
	configFile := fmt.Sprintf(`%s/%s`, file, "koiAliddns")
	if argsflag.show == "1" {
		if _, err := os.Stat(file); err != nil {
			if err := os.Mkdir(file, os.ModePerm); err != nil {
				stdErr.Error(err)
				os.Exit(1)
			}
		}
		if _, err := os.Stat(configFile); err != nil {
			f, err := os.Create(configFile)
			if err != nil {
				stdErr.Error(err)
				os.Exit(1)
			}
			defer f.Close()
			if _, err := f.WriteString(show); err != nil {
				stdErr.Error(err)
				os.Exit(1)
			} else {
				stdOut.Infof(`create %s successed`, configFile)
			}
		}
		os.Exit(0)
	}
}

func run() {
	stdOut.Info(`koifq alidns check start`)
	aksk, err := kuci.GetAKSK()
	if err != nil {
		stdErr.Error(err)
		return
	}
	ali := AKSK{
		AK: aksk.AK,
		SK: aksk.SK,
	}
	updates, adds, err := kuci.HostsHandler()
	if err != nil {
		stdErr.Error(err)
		return
	}
	switch {
	case len(updates) > 0:
		ali.UpdateDNS(updates)
	case len(adds) > 0:
		ali.AddDNS(adds)
	default:
		stdOut.Info(`koifq alidns check end, no new records`)
	}
}

func koiCron() {
	// uci.AppEnabled()
	koiAliddns, err := kuci.GetKoiAliDdns()
	if err != nil {
		stdErr.Error(err)
		return
	}
	if koiAliddns.Enabled == false {
		stdErr.Error(`app was not enabled to running, check config file`)
		os.Exit(1)
	}

	s := gocron.NewScheduler()
	s.Every(uint64(koiAliddns.Cron)).Seconds().Do(run)
	<-s.Start()
}

func main() {
	argsflag := parseFlags()
	file := argsflag.file

	kuci = NewUCI{
		UCI: uci.NewTree(file),
	}

	initconf(argsflag)

	koiCron()
	select {}
}
