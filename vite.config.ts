import { sveltekit } from "@sveltejs/kit/vite";
import { defineConfig } from "vite";
import { readFileSync } from "fs";
import { SvelteKitPWA } from "@vite-pwa/sveltekit";
const pkg = JSON.parse(readFileSync("package.json", "utf8"));

export default defineConfig({
	plugins: [
		sveltekit(),
		SvelteKitPWA({
			manifest: {
				name: "Watcharr",
				short_name: "Watcharr",
				description: "Your movie and show watched list.",
				background_color: "#f1da83",
				categories: ["entertainment"],
				start_url: "/",
				scope: "/",
				display: "standalone",
				icons: [
					{
						src: "/logo-sqre-144.png",
						sizes: "144x144",
						type: "image/png",
					},
					{
						src: "/logo-sqre-192.png",
						sizes: "192x192",
						type: "image/png",
					},
					{
						src: "/logo-sqre.png",
						sizes: "512x512",
						type: "image/png",
					},
					{
						src: "/logo-sqre.png",
						sizes: "512x512",
						type: "image/png",
						purpose: "any maskable",
					},
				],
			},
			devOptions: {
				// Off in dev on purpose. A precaching service worker in front of a
				// dev server means every asset change can leave a stale or broken
				// precache serving the app — at best confusing, at worst an
				// infinite reload when a precached URL stops existing.
				// Production builds still generate the service worker; only `vite
				// dev` is affected.
				enabled: false,
			},
		}),
	],
	define: {
		__WATCHARR_VERSION__: JSON.stringify(pkg.version),
	},
});
