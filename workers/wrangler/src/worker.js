/**
 * Worker C64: proxy HTTP al binary gopress-server arrancado via @cloudflare/sandbox.
 *
 * El binary Go (CGO: go-sqlite3 + quickjs-go) se ejecuta como proceso aislado
 * dentro del Worker; este Worker reenvía requests a :PORT y responde.
 *
 * Deploy: `wrangler deploy` (requiere wrangler 4.x + node). El binary se sube
 * como artifact o se buildea en CI (wrangler no buildea Go nativo).
 */
import { Sandbox } from "@cloudflare/sandbox";

export default {
  async fetch(request, env, ctx) {
    // Health check directo (cache) para evitar saturar el sandbox en checks de liveness.
    const url = new URL(request.url);
    if (url.pathname === "/healthz") {
      return new Response(JSON.stringify({ status: "ok", proxy: "gopress-cms" }), {
        headers: { "content-type": "application/json" },
      });
    }

    // El binary ya corre como proceso en :PORT (arrancado por sandbox en startup).
    const upstream = `http://127.0.0.1:${env.PORT ?? 8080}`;
    const target = new URL(url.pathname + url.search, upstream);

    const init = {
      method: request.method,
      headers: { ...request.headers },
      // body: request.body, // proxy streaming no esta disponible en este wrapper simplificado;
      // se delega al reverse proxy del platform en prod (Cloudflare + sandbox networking).
    };
    if (request.method !== "GET" && request.method !== "HEAD") {
      init.body = request.body;
    }

    try {
      const resp = await fetch(target, init);
      const out = new Response(resp.body, resp);
      out.headers.set("x-proxy", "gopress-cms-worker");
      return out;
    } catch (e) {
      return new Response("gopress upstream unreachable: " + e, { status: 502 });
    }
  },
};

// --- Sandbox binding (dev): arranca el binary como proceso durante el deploy ---
export const sandbox = new Sandbox({
  // El binary gopress-server.exe (o .exe segun arch) se buildea en CI:
  //   CGO_ENABLED=1 go build -o bin/gopress-server ./cmd/server
  // y se sube como artifact. Aqui arrancamos en modo dev con wrangler dev.
  command: "bin/gopress-server",
  env: {
    PORT: "8080",
    DB_PATH: "file:/data/gopress.db?cache=shared&mode=rwc&fk=1",
    DB_PATH_FALLBACK: ":memory:", // si /data no existe
    MIGRATIONS_DIR: "db/migrations",
    CMS_BODY_LIMIT: "524288",
  },
  // Expone el puerto del binary al Worker.
  ports: [8080],
});
