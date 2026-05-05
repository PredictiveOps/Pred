import NextAuth, { type NextAuthOptions } from "next-auth";

// Extend NextAuth Session type to include accessToken
declare module "next-auth" {
	interface Session {
		accessToken?: string;
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
			issuer: `http://localhost:8080/realms/${process.env.KEYCLOAK_REALM}`,
			clientId: process.env.KEYCLOAK_CLIENT_ID || "",
			clientSecret: process.env.KEYCLOAK_CLIENT_SECRET || "",
			authorization: {
				url: `http://localhost:8080/realms/${process.env.KEYCLOAK_REALM}/protocol/openid-connect/auth`,
				params: { scope: "openid profile email" },
			},
			token: `${process.env.KEYCLOAK_URL}/realms/${process.env.KEYCLOAK_REALM}/protocol/openid-connect/token`,
			userinfo: `${process.env.KEYCLOAK_URL}/realms/${process.env.KEYCLOAK_REALM}/protocol/openid-connect/userinfo`,
			jwks_endpoint: `${process.env.KEYCLOAK_URL}/realms/${process.env.KEYCLOAK_REALM}/protocol/openid-connect/certs`,
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
				secure: process.env.NODE_ENV === "production" && process.env.NEXTAUTH_URL?.startsWith("https://"),
				sameSite: "lax",
				path: "/",
				maxAge: 24 * 60 * 60,
			},
		},
		pkceCodeVerifier: {
			name: "next-auth.pkce.code_verifier",
			options: {
				httpOnly: true,
				secure: process.env.NODE_ENV === "production" && process.env.NEXTAUTH_URL?.startsWith("https://"),
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
			return session;
		},
	},
};
