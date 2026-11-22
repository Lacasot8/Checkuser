# CheckUser

Versão independente do CheckUser para Linux. O projeto fornece um serviço HTTP pequeno, um fallback local baseado em arquivo e um menu administrativo acessível pelo comando `check`. Ele não usa binário protegido, não altera instalações de terceiros e não depende do código interno do instalador de referência.

## Instalação

Em uma VPS Linux com systemd, como root:

```bash
wget -qO /tmp/install-checkuser.sh https://raw.githubusercontent.com/Lacasot8/Checkuser/main/install-checkuser.sh
chmod +x /tmp/install-checkuser.sh
bash /tmp/install-checkuser.sh
```

A instalação compila o serviço para `amd64` ou `arm64`, grava a configuração em `/etc/checkuser/checkuser.env`, cria `checkuser.service` e instala o painel em `/usr/local/bin/check`. Depois, abra o menu com `check`.

O instalador é idempotente. Se a configuração já existir, ela é preservada. O arquivo `/root/usuarios.db` nunca é removido pelo menu.

## Fonte local e endpoint

O fallback lê linhas no formato `usuario limite`, ignorando linhas vazias e comentários. Por exemplo:

```text
cliente01 30
cliente02 90
```

A consulta é feita por GET:

```text
http://IP-DA-VPS:2052/check?user=cliente01&uuid=UUID&hwid=HWID
```

A resposta é JSON e informa `ok`, `user`, `limit`, `source`, `message` e `checked_at`. O endpoint `/health` pode ser usado para verificar o serviço. Usuários desconhecidos retornam HTTP 404; parâmetros inválidos retornam 400.

## DragonCore e túnel

O serviço tenta consultar `CHECKUSER_DRAGONCORE_URL` e, se a fonte estiver disponível, devolve a resposta normalizada como `source: dragoncore`. Se DragonCore estiver indisponível, usa o arquivo local. Para desativar o upstream, deixe `CHECKUSER_DRAGONCORE_URL` vazio e reinicie o serviço.

O túnel Cloudflare é opcional. Se `cloudflared` estiver instalado em `/usr/local/bin/cloudflared`, a opção de atualização do menu inicia o serviço `checkuser-tunnel.service` e mostra o endereço `trycloudflare.com` encontrado nos logs. Para produção, prefira um túnel nomeado com domínio próprio; o quick tunnel é temporário.

## Configuração

As variáveis suportadas estão em `checkuser.env.example`. Após mudar `/etc/checkuser/checkuser.env`, execute `systemctl restart checkuser`.

## Remoção

Use `check`, escolha `7` e confirme digitando `REMOVER`. A remoção apaga somente o serviço, o binário, o menu e a configuração do CheckUser. O arquivo `/root/usuarios.db` é preservado.
