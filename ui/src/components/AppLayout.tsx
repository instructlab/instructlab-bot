// src/components/AppLayout.tsx
'use client';

import * as React from 'react';
import { useRouter } from 'next/navigation';
import { useSession } from 'next-auth/react';
import { Brand } from '@patternfly/react-core/dist/dynamic/components/Brand';
import { Button } from '@patternfly/react-core/dist/dynamic/components/Button';
import { Masthead } from '@patternfly/react-core/dist/dynamic/components/Masthead';
import { MastheadBrand } from '@patternfly/react-core/dist/dynamic/components/Masthead';
import { MastheadMain } from '@patternfly/react-core/dist/dynamic/components/Masthead';
import { MastheadToggle } from '@patternfly/react-core/dist/dynamic/components/Masthead';
import { MastheadContent } from '@patternfly/react-core/dist/dynamic/components/Masthead';
import { Nav } from '@patternfly/react-core/dist/dynamic/components/Nav';
import { NavItem } from '@patternfly/react-core/dist/dynamic/components/Nav';
import { NavList } from '@patternfly/react-core/dist/dynamic/components/Nav';
import { NavExpandable } from '@patternfly/react-core/dist/dynamic/components/Nav';
import { Page } from '@patternfly/react-core/dist/dynamic/components/Page';
import { PageSidebar } from '@patternfly/react-core/dist/dynamic/components/Page';
import { PageSidebarBody } from '@patternfly/react-core/dist/dynamic/components/Page';
import { SkipToContent } from '@patternfly/react-core/dist/dynamic/components/SkipToContent';
import { Spinner } from '@patternfly/react-core/dist/dynamic/components/Spinner';
import BarsIcon from '@patternfly/react-icons/dist/dynamic/icons/bars-icon';
import { useTheme } from '../context/ThemeContext';
import ThemeToggle from './ThemeToggle/ThemeToggle';
import Link from 'next/link';
import { signOut } from 'next-auth/react';

interface IAppLayout {
  children: React.ReactNode;
}

const AppLayout: React.FunctionComponent<IAppLayout> = ({ children }) => {
  const [sidebarOpen, setSidebarOpen] = React.useState(true);
  const { theme } = useTheme();
  const { data: session, status } = useSession();
  const router = useRouter();

  React.useEffect(() => {
    if (status === 'loading') return; // Do nothing while loading
    if (!session && router.pathname !== '/login') {
      router.push('/login'); // Redirect if not authenticated and not already on login page
    }
  }, [session, status, router]);

  if (status === 'loading') {
    return <Spinner />;
  }

  if (!session) {
    return null; // Return nothing if not authenticated to avoid flicker
  }

  const routes = [
    { path: '/dashboard', label: 'Dashboard' },
    {
      path: '/jobs',
      label: 'Jobs',
      children: [
        { path: '/jobs/all', label: 'All Jobs' },
        { path: '/jobs/running', label: 'Running Jobs' },
        { path: '/jobs/pending', label: 'Pending Jobs' },
        { path: '/jobs/failed', label: 'Failed Jobs' },
        { path: '/jobs/success', label: 'Successful Jobs' },
      ],
    },
    {
      path: '/contribute',
      label: 'Contribute',
      children: [
        { path: '/contribute/skill', label: 'Skill' },
        { path: '/contribute/knowledge', label: 'Knowledge' },
      ],
    },
    {
      path: '/playground',
      label: 'Laboratory',
      children: [
        { path: '/playground/chat', label: 'Chat' },
        { path: '/playground/ragchat', label: 'RAG Chat' },
        { path: '/playground/devchat', label: 'Developer Playground' },
        { path: '/playground/endpoints', label: 'Custom Model Settings' },
      ],
    },
  ];

  const Header = (
    <Masthead>
      <MastheadToggle>
        <Button variant="plain" onClick={() => setSidebarOpen(!sidebarOpen)} aria-label="Global navigation">
          <BarsIcon />
        </Button>
      </MastheadToggle>
      <MastheadMain>
        <MastheadBrand>
          <Brand src="/InstructLab-Logo.svg" alt="InstructLab Logo" heights={{ default: '60px' }} />
        </MastheadBrand>
      </MastheadMain>
      <MastheadContent className="masthead-right-align" style={{ width: '100%' }}>
        <div style={{ paddingLeft: '80%' }}>
          <ThemeToggle />
          {session ? (
            <Button onClick={() => signOut()} variant="primary">
              Logout
            </Button>
          ) : (
            <Link href="/login">Login</Link>
          )}
        </div>
      </MastheadContent>
    </Masthead>
  );

  const renderNavItem = (route: { path: string; label: string }, index: number) => (
    <NavItem key={`${route.label}-${index}`} id={`${route.label}-${index}`} isActive={route.path === router.pathname}>
      <Link href={route.path}>{route.label}</Link>
    </NavItem>
  );

  const renderNavExpandable = (
    route: {
      path: string;
      label: string;
      children: { path: string; label: string }[];
    },
    index: number
  ) => (
    <NavExpandable
      key={`${route.label}-${index}`}
      title={route.label}
      isActive={route.path === router.pathname || route.children.some((child) => child.path === router.pathname)}
      isExpanded
    >
      {route.children.map((child, idx) => renderNavItem(child, idx))}
    </NavExpandable>
  );

  const Navigation = (
    <Nav id="nav-primary-simple" theme={theme}>
      <NavList id="nav-list-simple">
        {routes.map((route, idx) => (route.children ? renderNavExpandable(route, idx) : renderNavItem(route, idx)))}
      </NavList>
    </Nav>
  );

  const Sidebar = (
    <PageSidebar theme={theme}>
      <PageSidebarBody>{Navigation}</PageSidebarBody>
    </PageSidebar>
  );

  const pageId = 'primary-app-container';
  const PageSkipToContent = <SkipToContent href={`#${pageId}`}>Skip to Content</SkipToContent>;

  return (
    <Page mainContainerId={pageId} header={Header} sidebar={sidebarOpen && Sidebar} skipToContent={PageSkipToContent}>
      {children}
    </Page>
  );
};

export { AppLayout };
