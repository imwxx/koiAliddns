# 1.koiAliddns

- **What**: Dynamic DNS Client scripts for Ali DNS, just support IPv4, config was suitable for OpenWRT
- **Cron**: Application running by 5Min each time

# 2.How
## 2.1.config
```
#create config like this
k@k-ThinkPad-P15-Gen-1:~$ /usr/bin/koiAliddns -c 1 -f /etc/config
{"level":"info","msg":"create /etc/config/koiAliddns successed","time":"2022-10-25T18:49:43+08:00"}
k@k-ThinkPad-P15-Gen-1:~$ cat /etc/config/koiAliddns

config koiAliddns
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

```

## 2.2.run
```
/usr/bin/koiAliddns -f /etc/config
```
