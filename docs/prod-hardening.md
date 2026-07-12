# Production hardening notes for public beta

## Token
COE_KA_ADMIN_TOKEN must be strong random (32+ bytes). Never commit. Rotate on leak.

## TLS
Use Caddy / nginx / cloud LB with public cert (Let's Encrypt). Never -http in prod.

## TURN
Run coturn or provider. Agents set:
COE_TURN_URLS=turn:turn.yourdomain:3478
COE_TURN_USER=...
COE_TURN_PASS=...

## Signing (Windows)
Use signtool + EV or code-sign cert. Run:
.\scripts\build-release.ps1 -Version x.y.z -Sign -CertThumb <thumb>

## Backup
Regularly backup /data/master.key and registry.json. Test restore.

## Monitoring
ka-check in cron. Alert on fail or cert expiry <14d.
