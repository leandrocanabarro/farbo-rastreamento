# Configurando o rastreador

Roteiro para colocar um aparelho para conversar com a plataforma.

> Os comandos abaixo variam por modelo e firmware. **Confira no manual do seu
> aparelho.** O que está aqui são os formatos mais comuns da linha TKSTAR e da
> linha Concox/GT06 — não uma garantia de que valem para o seu.

---

## 1. Antes de tudo

- [ ] Chip M2M/dados ativo, com SMS habilitado
- [ ] IMEI anotado (15 dígitos, na etiqueta do aparelho)
- [ ] IP público ou domínio do servidor
- [ ] Porta TCP (`TCP_PORT`, padrão `5000`) aberta na internet
- [ ] IMEI **já cadastrado** no painel

O último item não é detalhe: tráfego de IMEI desconhecido é recusado e a
sessão encerrada. É proposital — evita que qualquer aparelho perdido no mundo
encha seu banco.

---

## 2. Configuração por SMS

Mande os comandos para o número do chip do rastreador. `123456` é a senha
padrão de fábrica da maioria; **troque-a**.

### Linha TKSTAR (TK905, TK915, TK910, TK970)

```text
begin123456                          inicializa
adminip123456 <IP> 5000              servidor e porta
apn123456 <apn-da-operadora>         APN
upload123456 30                      intervalo em segundos
check123456                          conferir a configuração
```

APNs comuns no Brasil:

| Operadora | APN | Usuário | Senha |
| --- | --- | --- | --- |
| Vivo | `zap.vivo.com.br` | `vivo` | `vivo` |
| Claro | `claro.com.br` | `claro` | `claro` |
| TIM | `timbrasil.br` | `tim` | `tim` |
| Oi | `gprs.oi.com.br` | `oi` | `oi` |
| Arqia / M2M | conforme o contrato | | |

Com APN autenticada:

```text
apnuser123456 vivo
apnpasswd123456 vivo
```

### Linha Concox / GT06

```text
SERVER,1,<HOST_OU_IP>,5000,0#
APN,<apn>,<usuario>,<senha>#
TIMER,30#
PARAM#                               conferir a configuração
```

O painel monta esses textos para você, já com host e porta preenchidos:
**Rastreadores → Configuração**. Ele apenas mostra — nada é enviado
automaticamente ao aparelho.

---

## 3. Instalação com relé de corte

O corte de motor precisa de um relé ligado à ignição ou à bomba de
combustível. Isso é serviço de instalador; três coisas valem dizer:

- **Corte a ignição, não a bomba**, quando houver escolha. Interromper a bomba
  com o motor em rotação é pior.
- **O relé precisa ser normalmente fechado** no circuito, para que uma falha do
  rastreador não deixe o carro sem partida.
- **Teste parado, em lugar seguro**, antes de confiar no sistema.

A plataforma recusa o corte acima de `ENGINE_CUT_MAX_SPEED_KMH` (padrão
5 km/h) e com posição mais velha que `ENGINE_CUT_MAX_POSITION_AGE`. O firmware
dos aparelhos com relé costuma ter a própria proteção. **As duas somam; nenhuma
substitui bom senso na instalação.**

---

## 4. Conferindo que funcionou

```bash
# Chegou alguma conexão?
docker compose logs -f backend | grep -i "sessão de rastreador"

# Sessões abertas agora
curl -H "Authorization: Bearer <token>" \
  http://localhost:8080/api/diagnostics/connections
```

No painel: o veículo aparece na lista com o selo **Online** e o mapa centraliza
nele.

---

## 5. TK915 e a linha magnética

O TK915 é rastreador de patrimônio, não de bloqueio:

- **Não tem relé.** Desligar/liberar motor não existe nesse aparelho. O comando
  seria aceito e não faria nada. Corte nessa família é para os modelos com
  relé (TK920, TK806) — ou, com suporte melhor testado, para a linha
  Concox/GT06.
- **É 2G.** A Anatel parou de certificar aparelhos só-2G/3G em abril de 2025 e
  o desligamento vai até 2028. Para instalação nova, prefira 4G.
- **Reporta um identificador curto**, de 10 dígitos, não o IMEI de 15. Cadastre
  exatamente o que ele manda (ver seção 6).

Para o que ele serve, serve bem: ímã, sem instalação, meses de bateria. O
adaptador `h02` atende ele para rastreamento.

---

## 6. Descobrindo o identificador de um aparelho H02

```bash
docker compose logs backend | grep "não está cadastrado"
```

Se o identificador aparecer mascarado, ponha `LOG_MASK_IMEI=false` no `.env`,
reinicie o backend e repita. Cadastre o valor exato que aparecer.

---

## 7. Quando não funciona

| Sintoma | Provável causa | O que fazer |
| --- | --- | --- |
| Nenhuma conexão chega | porta fechada ou IP errado | teste `nc -vz <ip> 5000` de fora da rede; confira NAT e firewall |
| Conecta e cai na hora | IMEI não cadastrado | cadastre o IMEI; o log mostra o IMEI mascarado que tentou |
| Conecta e nada aparece | protocolo não reconhecido | ligue `TKSTAR_V4_CAPTURE=true` e veja **Diagnóstico → Pacotes não interpretados** |
| Posição no meio do oceano | coordenadas em 0,0 | o aparelho está sem fix de GPS; leve-o para céu aberto |
| Só heartbeat, sem posição | sem fix ou intervalo alto demais | confira `upload`/`TIMER` e a antena |
| Comando não chega | aparelho offline | comando só sai com sessão aberta; veja o status no painel |
| Comando volta `Fail` | senha de comando | preencha a senha no cadastro do rastreador |
| Corte sempre recusado | velocidade ou posição velha | veja o motivo exato na resposta e em **Diagnóstico → Auditoria** |

### Lendo a captura

Em **Diagnóstico → Pacotes não interpretados**, olhe a coluna ASCII:

```text
*HQ,869247061234567,V1,134530,A,...    → H02 texto → h02
$..... (ilegível), hex começa 24       → H02 binário → h02
imei:869247061234567,tracker,...       → família GPS103 → tkstar_v1
xx..xx (ilegível), hex começa 7878     → GT06 → veja o erro no log
outra coisa                            → parser novo: docs/PROTOCOLS.md
```

---

## 8. Trocando o texto de um comando

Se o seu firmware usa um texto diferente do padrão — por exemplo `RELAY,1#` no
lugar de `DYD#` —, não é preciso recompilar nada. Cadastre a substituição:

```bash
curl -X PATCH http://localhost:8080/api/devices/<id> \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "imei": "869247061234567",
    "commandOverrides": {
      "ENGINE_CUT": "RELAY,1#",
      "ENGINE_RESUME": "RELAY,0#"
    }
  }'
```

O texto cadastrado passa a ser usado e aparece igual na auditoria.
