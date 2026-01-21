import { getAllDocsSlugs, getDocData } from "@/lib/docs";
import ReactMarkdown from "react-markdown";
import { Button } from "@/components/ui/button";
import { ArrowLeft } from "lucide-react";
import Link from "next/link";

export async function generateStaticParams() {
  const slugs = getAllDocsSlugs();
  return slugs;
}

export default async function DocPage({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const docData = getDocData(slug);

  if (!docData) {
    return <div>Doc not found</div>;
  }

  return (
    <main className="relative min-h-screen flex flex-col items-center py-20 px-4 bg-background">
        <div className="max-w-4xl w-full space-y-8 relative z-10">
            <div className="flex items-center gap-4">
                <Button variant="outline" size="icon" asChild>
                    <Link href="/docs">
                        <ArrowLeft className="h-4 w-4" />
                    </Link>
                </Button>
                <h1 className="text-3xl font-bold capitalize">{docData.slug.replace(/_/g, ' ')}</h1>
            </div>
            
            <article className="prose prose-zinc dark:prose-invert max-w-none">
                <ReactMarkdown>{docData.content}</ReactMarkdown>
            </article>
        </div>
    </main>
  );
}
