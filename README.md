alum
====

A forwarding mail server inspired by @alum.mit.edu


## What's in the package

* A secure Postfix instance that will ONLY forward mail, according to the aliases file
* Full opportunistic TLS support
* Automatic security updates, with reboot, and emails on error to `postmaster@alum.example.com`
* Daily Tarsnap backups of the aliases file
* An *A* grade HTTPS nginx reverse proxy pointing to `http://localhost:8000`
* A Ansible playbook to setup all of this


## Usage

* Get a SSL certificate, and put 2 copies of the concatenated PEM key and certificate in the root of the repo, named `smtpd.pem` and `nginx.pem` (the order is important: private key, your certificate, other certificates)
* Create a Tarsnap key and put it in `tarsnap.key`
* Start a Ubuntu 14.04 LTS machine
* Make sure you can ssh into the machine, and that sudo is passwordless
* Create a `inventory.ini` file like this
```
[alum]
98.25.536.22
```
* Run
```
ansible-playbook -i inventory.ini \
    -e hostname=alum.example.com \
    -e domain=alum.example.com \
    playbook.yml
```
* Set the DNS A record for `alum.example.com` to point to the machine, and the MX record of `alum.example.com` to `alum.example.com`


## Adding aliases

Add them like this

```
postmaster@alum.example.com me@example.com
joe@alum.example.com joe@gmail.com
<alias-email-with-domain> <actual-recipient-email>
```

to `/etc/postfix/virtual` and then run

```sh
# postmap /etc/postfix/virtual
# postfix reload
```
