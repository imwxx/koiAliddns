# koiAliddns

- **What**: Dynamic DNS Client scripts for Ali DNS, just support IPv4, config was suitable for OpenWRT
- **Cron**: Application running by 5Min each time

# How
## config
```
#create config like this
#k@k-ThinkPad-P15-Gen-1:~$ cat /etc/config/koiq
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

```
## run
```
/usr/bin/koiAliddns -f /etc/config
```
