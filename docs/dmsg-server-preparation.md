## This document depicts the process to run standalone dmsg-server as a Systemd service.


To run a server as a systemd process, we have to have systemd service at ```/etc/systemd/system/```  path. 

Follow the below steps to create one.

*Prerequisites:
    - We assume you have gone through [Local-setup](https://github.com/skycoin/dmsg/blob/master/integration/README.md) once. also you have ```Redis``` installed in system.


1. create dmsg.service under ```/etc/systemd/system/dmsg.service```

```

[Unit]

# Description of the service
Description=DMSG demo service
# The services need to run before this service.
After=redis.service
After=network.target
StartLimitIntervalSec=0

[Service]
Type=simple
Restart=always
RestartSec=1
# The actual logged in username
User=skycoin

# Working dir or cloned repo path
WorkingDirectory=/home/skycoin/Documents/Sky/dmsg/

# Go ENV
Environment="PATH=/usr/local/go/bin"

# After make build, Path to built binary
ExecStart=/home/skycoin/Documents/Sky/dmsg/bin/dmsg-server /home/skycoin/Documents/Sky/dmsg/integration/configs/dmsgserver1.json

[Install]
WantedBy=multi-user.target


```

After this you can run below commands to test.

- ```systemctl restart dmsg```

- ```systemctl status dmsg```

check if its running and status is ```active```

you can verify the ```dmsg discovery``` service running as in same way, or the one describe in [here](https://github.com/skycoin/dmsg/blob/master/integration/README.md)

You should see the integration between two.
