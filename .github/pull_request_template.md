## Qué cambia

<!-- Una o dos líneas. -->

## Checklist

- [ ] `git rebase origin/main` hecho justo antes de abrir el PR
- [ ] `git diff --stat origin/main` solo muestra carpetas propias
- [ ] No toca `shared/`, `openapi.yaml`, `main.go`, `app.routes.ts`, `docker-compose.yml`, `AGENTS.md` ni dependencias (si lo hace, es un PR aparte y avisé al grupo)
- [ ] Migraciones nuevas con mi prefijo (`1xxx_`, `2xxx_`, `3xxx_`); no edité la `0001`
- [ ] `gofmt` y pruebas pasan localmente
