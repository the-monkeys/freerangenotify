import React, { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { oneLight } from 'react-syntax-highlighter/dist/esm/styles/prism';
import { Skeleton } from '../../components/ui/skeleton';

// Vite's import.meta.glob with ?raw loads .md files as strings at build time.
// `eager: false` makes them lazy-loaded chunks.
const modules = import.meta.glob('../../docs/*.md', { query: '?raw', import: 'default' }) as Record<string, () => Promise<string>>;

function resolveModule(slug: string): (() => Promise<string>) | undefined {
    // Match against the glob keys, e.g. "../../docs/getting-started.md"
    const key = Object.keys(modules).find(k => k.endsWith(`/${slug}.md`));
    return key ? modules[key] : undefined;
}

const DocsPage: React.FC = () => {
    const { slug } = useParams<{ slug: string }>();
    const [content, setContent] = useState<string | null>(null);
    const [loading, setLoading] = useState(true);
    const [notFound, setNotFound] = useState(false);

    useEffect(() => {
        if (!slug) return;
        setLoading(true);
        setNotFound(false);

        const loader = resolveModule(slug);
        if (!loader) {
            setNotFound(true);
            setLoading(false);
            return;
        }

        loader()
            .then(md => {
                setContent(md);
                setLoading(false);
            })
            .catch(() => {
                setNotFound(true);
                setLoading(false);
            });
    }, [slug]);

    if (loading) {
        return (
            <div className="space-y-4">
                <Skeleton className="h-8 w-3/4" />
                <Skeleton className="h-4 w-full" />
                <Skeleton className="h-4 w-5/6" />
                <Skeleton className="h-4 w-full" />
                <Skeleton className="h-4 w-4/5" />
                <Skeleton className="h-6 w-1/2 mt-6" />
                <Skeleton className="h-4 w-full" />
                <Skeleton className="h-4 w-3/4" />
            </div>
        );
    }

    if (notFound || !content) {
        return (
            <div className="text-center py-16 space-y-4">
                <h2 className="text-lg font-semibold">Document not found</h2>
                <p className="text-sm text-muted-foreground">
                    The page <code className="bg-muted px-1.5 py-0.5 rounded text-xs">{slug}</code> doesn't exist.
                </p>
                <Link to="/docs/getting-started" className="text-sm text-accent hover:underline">
                    Go to Getting Started →
                </Link>
            </div>
        );
    }

    return (
        <article className="prose-custom">
            <ReactMarkdown
                remarkPlugins={[remarkGfm]}
                components={{
                    h1: ({ children }) => <h1 className="text-2xl font-bold mb-4 text-foreground">{children}</h1>,
                    h2: ({ children }) => <h2 className="text-xl font-semibold mt-8 mb-3 border-b border-border pb-2 text-foreground">{children}</h2>,
                    h3: ({ children }) => <h3 className="text-lg font-medium mt-6 mb-2 text-foreground">{children}</h3>,
                    p: ({ children }) => <p className="text-sm text-foreground/80 leading-relaxed mb-4">{children}</p>,
                    a: ({ href, children }) => (
                        <a href={href} className="text-accent hover:underline" target={href?.startsWith('http') ? '_blank' : undefined} rel="noopener noreferrer">
                            {children}
                        </a>
                    ),
                    ul: ({ children }) => <ul className="list-disc pl-6 space-y-1 text-sm text-foreground/80 mb-4">{children}</ul>,
                    ol: ({ children }) => <ol className="list-decimal pl-6 space-y-1 text-sm text-foreground/80 mb-4">{children}</ol>,
                    li: ({ children }) => <li className="leading-relaxed">{children}</li>,
                    blockquote: ({ children }) => (
                        <blockquote className="border-l-4 border-accent pl-4 italic text-muted-foreground my-4">{children}</blockquote>
                    ),
                    table: ({ children }) => (
                        <div className="overflow-x-auto mb-4">
                            <table className="w-full text-sm border-collapse border border-border">{children}</table>
                        </div>
                    ),
                    thead: ({ children }) => <thead className="bg-muted">{children}</thead>,
                    th: ({ children }) => <th className="border border-border px-3 py-2 text-left font-medium text-foreground">{children}</th>,
                    td: ({ children }) => <td className="border border-border px-3 py-2 text-foreground/80">{children}</td>,
                    code: ({ className, children, ...props }) => {
                        const match = /language-(\w+)/.exec(className || '');
                        if (match) {
                            return (
                                <SyntaxHighlighter
                                    style={oneLight}
                                    language={match[1]}
                                    PreTag="div"
                                    className="rounded-lg text-xs my-4 border border-border !bg-muted"
                                >
                                    {String(children).replace(/\n$/, '')}
                                </SyntaxHighlighter>
                            );
                        }
                        return (
                            <code className="bg-muted px-1.5 py-0.5 rounded text-xs font-mono" {...props}>
                                {children}
                            </code>
                        );
                    },
                    pre: ({ children }) => <>{children}</>,
                    hr: () => <hr className="my-8 border-border" />,
                    strong: ({ children }) => <strong className="font-semibold text-foreground">{children}</strong>,
                }}
            >
                {content}
            </ReactMarkdown>
        </article>
    );
};

export default DocsPage;
