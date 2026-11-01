import { DocsLayout } from 'fumadocs-ui/layouts/docs';
import { baseOptions } from '@/lib/layout.shared';
import { referenceTree } from '@/lib/trees';

export default function Layout({ children }: LayoutProps<'/'>) {
  return (
    <DocsLayout tree={referenceTree()} {...baseOptions()}>
      {children}
    </DocsLayout>
  );
}
