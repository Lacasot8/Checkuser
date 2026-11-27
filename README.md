# CheckUser

Versão independente do CheckUser para Linux. O projeto fornece um serviço HTTP pequeno, um fallback local baseado em arquivo e um menu administrativo acessível pelo comando `ck`. O instalador usa nomes exclusivos para seus serviços e não altera o comando `check` nem serviços de outras instalações.

## Instalação

Em uma VPS Linux com systemd, como root:

```bash
wget -qO /tmp/install-checkuser.sh https://raw.githubusercontent.com/Lacasot8/Checkuser/main/install-checkuser.sh
chmod +x /tmp/install-checkuser.sh
bash /tmp/install-checkuser.sh
```

A instalação compila o serviço para `amd64` ou `arm64`, instala o `cloudflared` se necessário, grava a configuração em `/etc/checkuser/checkuser.env`, cria os serviços exclusivos `checkuser-lacasot8.service` e `checkuser-lacasot8-tunnel.service`, e instala o painel em `/usr/local/bin/ck`. A porta padrão é `2053`. No menu `ck`, a opção `[8] Alterar porta` permite escolher outra porta, incluindo `2052` quando estiver livre; o menu recusa a mudança se detectar outro processo escutando nela. O comando `check` e os serviços de outras instalações são preservados.

O instalador é idempotente. Se a configuração já existir, ela é preservada. O arquivo `/root/usuarios.db` nunca é removido pelo menu.

## Fonte local e endpoint

O fallback lê linhas no formato `usuario limite`, ignorando linhas vazias e comentários. Por exemplo:

```text
cliente01 30
cliente02 90
```

A consulta é feita por GET:

```text
http://IP-DA-VPS:PORTA/check?username=cliente01&uuid=UUID&hwid=HWID
```

O CheckUser também aceita a rota usada pelo cliente original do DTunnel, com o usuário no caminho e o ID do aparelho em `deviceId`:

```text
http://IP-DA-VPS:PORTA/check/cliente01?deviceId=ID_DO_APARELHO
```

Para configurar o DTunnel, use `https://SEU-LINK.trycloudflare.com/check/{username}?deviceId={deviceId}`. A resposta é JSON (`application/json`) e inclui `username`, `expiration_date`, `expiration_days`, `count_connections` e `limit_connections`. Quando o DragonCore responder, os campos originais são preservados. Também é enviado `message_html` neste formato, para clientes que aceitam HTML:

```html
👤 Nome de usuario: {username}<br>📆 Expira em: {expiration_date}<br>📅 Dias restantes: {expiration_days}<br>🚫 Conexoes: {count_connections}|{limit_connections}
```

No fallback local, o limite vem de `/root/usuarios.db`, a expiração vem da data de expiração da conta Linux em `/etc/shadow` (o mesmo dado exibido por `chage -l`) e as conexões são contadas a partir de processos SSH/Dropbear do usuário. Se a conta Linux não tiver data de expiração ou sessão ativa, os valores correspondentes ficam vazios ou zero. O endpoint `/health` pode ser usado para verificar o serviço. Usuários desconhecidos retornam HTTP 404; parâmetros inválidos retornam 400.

## DragonCore e túnel

O serviço tenta consultar `CHECKUSER_DRAGONCORE_URL` e, se a fonte estiver disponível, repassa a resposta original do DragonCore, seja JSON, HTML ou texto. Se DragonCore estiver indisponível, usa o arquivo local. Para desativar o upstream, deixe `CHECKUSER_DRAGONCORE_URL` vazio e reinicie o serviço.

O instalador baixa `cloudflared` para `/usr/local/bin/cloudflared` somente se ainda não estiver instalado e inicia um Quick Tunnel apontando à porta configurada no arquivo de ambiente. O menu mostra o domínio público e a rota `/check`. Esse endereço é temporário e muda quando o túnel é reiniciado; para produção, prefira um túnel nomeado com domínio próprio.

## Configuração

As variáveis suportadas estão em `checkuser.env.example`. Após mudar `/etc/checkuser/checkuser.env`, execute `systemctl restart checkuser-lacasot8.service`.

## Remoção

Use `ck`, escolha `7` e confirme digitando `REMOVER`. A remoção apaga somente os serviços `checkuser-lacasot8*`, o binário, o menu `ck` e a configuração em `/etc/checkuser`. O arquivo `/root/usuarios.db`, o binário compartilhado `cloudflared`, o comando `check` e os serviços de outras instalações são preservados.
