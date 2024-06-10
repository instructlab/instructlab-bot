// src/app/api/auth/[...nextauth]/route.ts
import NextAuth, { NextAuthOptions } from 'next-auth';
import GitHubProvider from 'next-auth/providers/github';
import CredentialsProvider from 'next-auth/providers/credentials';
import axios from 'axios';
import winston from 'winston';
import path from 'path';

// Extend the Session and JWT types
declare module 'next-auth' {
  interface Session {
    accessToken?: string;
    id?: string;
  }

  interface User {
    id?: string;
  }
}

declare module 'next-auth/jwt' {
  interface JWT {
    accessToken?: string;
    id?: string;
  }
}

// Logger setup
const logger = winston.createLogger({
  level: 'info',
  format: winston.format.combine(
    winston.format.timestamp(),
    winston.format.printf(({ timestamp, level, message }) => {
      return `${timestamp} ${level}: ${message}`;
    })
  ),
  transports: [new winston.transports.Console(), new winston.transports.File({ filename: path.join(process.cwd(), 'auth.log') })],
});

const ORG = 'instructlab';

const authOptions: NextAuthOptions = {
  providers: [
    GitHubProvider({
      clientId: process.env.OAUTH_GITHUB_ID!,
      clientSecret: process.env.OAUTH_GITHUB_SECRET!,
      authorization: { params: { scope: 'public_repo' } },
    }),
    CredentialsProvider({
      name: 'Credentials',
      credentials: {
        username: { label: 'Username', type: 'text' },
        password: { label: 'Password', type: 'password' },
      },
      authorize: async (credentials) => {
        if (
          credentials?.username === (process.env.IL_UI_ADMIN_USERNAME || 'admin') &&
          credentials?.password === (process.env.IL_UI_ADMIN_PASSWORD || 'password')
        ) {
          logger.info(`User ${credentials.username} logged in successfully with credentials`);
          return { id: '1', name: 'Admin', email: 'admin@example.com' };
        }
        logger.warn(`Failed login attempt with username: ${credentials?.username}`);
        return null;
      },
    }),
  ],
  secret: process.env.NEXTAUTH_SECRET,
  session: {
    strategy: 'jwt',
  },
  callbacks: {
    async jwt({ token, user, account }) {
      if (account) {
        token.accessToken = account.access_token!;
      }
      if (user) {
        token.id = user.id;
      }
      console.log('JWT Callback:', token);
      return token;
    },
    async session({ session, token }) {
      if (token) {
        session.accessToken = token.accessToken;
        session.id = token.id;
      }
      console.log('Session Callback:', session);
      return session;
    },
    async signIn({ account, profile }) {
      if (account && account.provider === 'github' && profile) {
        const githubProfile = profile as { login: string };

        try {
          const response = await axios.get(`https://api.github.com/orgs/${ORG}/members/${githubProfile.login}`, {
            headers: {
              Accept: 'application/vnd.github+json',
              Authorization: `Bearer ${process.env.GITHUB_TOKEN}`,
              'X-GitHub-Api-Version': '2022-11-28',
            },
            validateStatus: (status) => {
              return [204, 302, 404].includes(status);
            },
          });

          if (response.status === 204) {
            console.log(`User ${githubProfile.login} logged in successfully with GitHub`);
            logger.info(`User ${githubProfile.login} logged in successfully with GitHub`);
            return true;
          } else if (response.status === 404) {
            console.log(`User ${githubProfile.login} is not a member of the ${ORG} organization`);
            logger.warn(`User ${githubProfile.login} is not a member of the ${ORG} organization`);
            return `/error?error=AccessDenied`; // Redirect to custom error page
          } else {
            console.log(`Unexpected error for user ${githubProfile.login} during organization membership verification`);
            logger.error(`Unexpected error for user ${githubProfile.login} during organization membership verification`);
            return false;
          }
        } catch (error) {
          if (axios.isAxiosError(error)) {
            logger.error(`Error fetching GitHub organization membership for user ${githubProfile.login}: ${error.message}`, {
              url: error.config?.url,
              method: error.config?.method,
              data: error.response?.data,
              status: error.response?.status,
            });
          } else {
            logger.error(`Error fetching GitHub organization membership for user ${githubProfile.login}: ${error}`);
          }
          return false;
        }
      }
      return true;
    },
  },
  pages: {
    signIn: '/login',
    error: '/error',
  },
};

const handler = NextAuth(authOptions);

export { handler as GET, handler as POST };
