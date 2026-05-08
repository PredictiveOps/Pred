import NextAuth, { type NextAuthOptions } from "next-auth";
import { extractTenantId } from "./jwt";

// Extend NextAuth Session type to include accessToken and tenantId
declare module "next-auth" {
	interface Session {
		accessToken?: string;
		tenantId?: number | null;
		user?: {
			name?: string | null;
			email?: string | null;
			image?: string | null;
		};
	}
}

export const authOptions: NextAuthOptions = {
	debug: true,
	secret: process.env.NEXTAUTH_SECRET,
	providers: [
		{
			id: "keycloak",
			name: "Keycloak",
			type: "oauth",
			// Don't use `wellKnown` — openid-client would discover endpoints from
			// Keycloak and use them verbatim, which breaks split-horizon dev setups
			// (Keycloak advertises localhost:8080 for everything because
			// KC_HOSTNAME=localhost). Provide endpoints manually instead so the
			// browser uses the public URL for /auth while the server uses the
			// internal Docker hostname for /token, /userinfo, and JWKS.
			issuer: `${process.env.KEYCLOAK_PUBLIC_URL}/realms/${process.env.KEYCLOAK_REALM}`,
			authorization: {
				url: `${process.env.KEYCLOAK_PUBLIC_URL}/realms/${process.env.KEYCLOAK_REALM}/protocol/openid-connect/auth`,
				params: { scope: "openid profile email tenant" },
			},
			token: `${process.env.KEYCLOAK_URL}/realms/${process.env.KEYCLOAK_REALM}/protocol/openid-connect/token`,
			userinfo: `${process.env.KEYCLOAK_URL}/realms/${process.env.KEYCLOAK_REALM}/protocol/openid-connect/userinfo`,
			jwks_endpoint: `${process.env.KEYCLOAK_URL}/realms/${process.env.KEYCLOAK_REALM}/protocol/openid-connect/certs`,
			clientId: process.env.KEYCLOAK_CLIENT_ID || "",
			clientSecret: process.env.KEYCLOAK_CLIENT_SECRET || "",
			idToken: true,
			profile(profile: any) {
				return {
					id: profile.sub || profile.user_id || profile.id,
					name: profile.name || profile.preferred_username,
					email: profile.email,
					image: profile.picture,
				};
			},
		} as any,
	],
	pages: {
		signIn: "/login",
		error: "/login",
	},
	cookies: {
		state: {
			name: "next-auth.state",
			options: {
				httpOnly: true,
				secure: process.env.NODE_ENV === "production",
				sameSite: "lax",
				path: "/",
				maxAge: 24 * 60 * 60,
			},
		},
		pkceCodeVerifier: {
			name: "next-auth.pkce.code_verifier",
			options: {
				httpOnly: true,
				secure: process.env.NODE_ENV === "production",
				sameSite: "lax",
				path: "/",
				maxAge: 24 * 60 * 60,
			},
		},
	},
	callbacks: {
		async jwt({ token, account, profile }) {
			if (account) {
				token.accessToken = account.access_token;
				token.refreshToken = account.refresh_token;
				token.expiresAt = account.expires_at;
				token.provider = account.provider;
			}
			return token;
		},
		async session({ session, token }) {
			session.accessToken = token.accessToken as string;
			session.tenantId = token.accessToken
				? extractTenantId(token.accessToken as string)
				: null;
			return session;
		},
	},
};
