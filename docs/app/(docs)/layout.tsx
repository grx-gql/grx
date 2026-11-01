import { DocsLayout } from 'fumadocs-ui/layouts/docs';
import { baseOptions } from '@/lib/layout.shared';
import { docsTree } from '@/lib/trees';

export default function Layout({ children }: LayoutProps<'/'>) {
  return (
    <DocsLayout tree={docsTree()} {...baseOptions()}>
      {children}
    </DocsLayout>
  );
}
