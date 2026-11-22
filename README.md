# CheckUser

Versão independente do CheckUser para Linux. O projeto fornece um serviço HTTP pequeno, um fallback local baseado em arquivo e um menu administrativo acessível pelo comando `check`. Ele não usa binário protegido, não altera instalações de terceiros e não depende do código interno do instalador de referência.

## Instalação

Em uma VPS Linux com systemd, como root:

```bash
wget -qO /tmp/install-checkuser.sh https://raw.githubusercontent.com/Lacasot8/Checkuser/main/install-checkuser.sh
chmod +x /tmp/install-checkuser.sh
bash /tmp/install-checkuser.sh
```

A instalação compila o serviço para `amd64` ou `arm64`, instala o `cloudflared`, grava a configuração em `/etc/checkuser/checkuser.env`, cria `check-dt-voidpro.service` e `check-dt-voidpro-tunnel.service`, e instala o painel em `/usr/local/bin/check`. O código usa APIs compatíveis com Go 1.13 ou superior. Depois, abra o menu com `check`.

O instalador é idempotente. Se a configuração já existir, ela é preservada. O arquivo `/root/usuarios.db` nunca é removido pelo menu.

## Fonte local e endpoint

O fallback lê linhas no formato `usuario limite`, ignorando linhas vazias e comentários. Por exemplo:

```text
cliente01 30
cliente02 90
```

A consulta é feita por GET:

```text
http://IP-DA-VPS:2052/check?username=cliente01&uuid=UUID&hwid=HWID
```

A resposta é JSON e aceita `username` (ou o alias `user`), `uuid` e `hwid`. Quando o DragonCore responder, o CheckUser preservará os campos reais do painel, incluindo `username`, `expiration_date`, `expiration_days`, `count_connections` e `limit_connections`. Também envia `message_html` neste formato, para clientes que aceitam HTML:

```html
👤 Nome de usuario: {username}<br>📆 Expira em: {expiration_date}<br>📅 Dias restantes: {expiration_days}<br>🚫 Conexoes: {count_connections}|{limit_connections}
```

No fallback local, o limite vem de `/root/usuarios.db`, a expiração vem da data de expiração da conta Linux em `/etc/shadow` (o mesmo dado exibido por `chage -l`) e as conexões são contadas a partir de processos SSH/Dropbear do usuário. Se a conta Linux não tiver data de expiração ou sessão ativa, os valores correspondentes ficam vazios ou zero. O endpoint `/health` pode ser usado para verificar o serviço. Usuários desconhecidos retornam HTTP 404; parâmetros inválidos retornam 400.

## DragonCore e túnel

O serviço tenta consultar `CHECKUSER_DRAGONCORE_URL` e, se a fonte estiver disponível, repassa a resposta original do DragonCore, seja JSON, HTML ou texto. Se DragonCore estiver indisponível, usa o arquivo local. Para desativar o upstream, deixe `CHECKUSER_DRAGONCORE_URL` vazio e reinicie o serviço.

O instalador baixa `cloudflared` para `/usr/local/bin/cloudflared` e inicia um Quick Tunnel com `cloudflared tunnel --url http://127.0.0.1:2052`. O menu mostra primeiro somente o domínio público, como `https://nome.trycloudflare.com`, e em uma linha separada mostra a rota `/check`. Esse endereço é temporário e muda quando o túnel é reiniciado; para produção, prefira um túnel nomeado com domínio próprio.

## Configuração

As variáveis suportadas estão em `checkuser.env.example`. Após mudar `/etc/checkuser/checkuser.env`, execute `systemctl restart checkuser`.

## Remoção

Use `check`, escolha `7` e confirme digitando `REMOVER`. A remoção apaga somente o serviço, o binário, o menu e a configuração do CheckUser. O arquivo `/root/usuarios.db` é preservado.
