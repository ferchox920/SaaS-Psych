# SessionFlow Web

Frontend web del MVP profesional/admin de SessionFlow.

## Stack

- Next.js App Router
- TypeScript
- Tailwind CSS v4
- shadcn/ui como base de componentes
- TanStack Query para server state
- React Hook Form + Zod para formularios

## Estructura

- `src/app`: routing, layouts y entrypoints de pantallas.
- `src/features`: UI, hooks y acceso a datos organizados por feature.
- `src/components`: shell compartido y componentes UI base.
- `src/lib`: cliente HTTP, config y utilidades transversales.
- `src/providers`: providers globales.

## Desarrollo local

1. Copiar variables:

```powershell
Copy-Item .env.example .env.local
```

2. Instalar dependencias desde la raiz del repo:

```powershell
corepack pnpm install
```

3. Levantar el frontend:

```powershell
corepack pnpm --filter web dev
```

App: [http://localhost:3000](http://localhost:3000)  
API esperada: [http://localhost:8080/api/v1](http://localhost:8080/api/v1)

## Credenciales demo actuales

- Tenant demo owner: `11111111-1111-1111-1111-111111111111`
- Usuario: `owner@tenant-a.local`
- Password: `ChangeMe123!`
