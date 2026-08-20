interface AppSplashProps {
  caption?: string;
}

export default function AppSplash({ caption }: AppSplashProps) {
  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <div className="flex flex-col items-center gap-4">
        <img
          src="/gaggle-goose.png"
          alt="Gaggle"
          width={160}
          height={160}
          fetchPriority="high"
          decoding="async"
          className="h-16 w-16 rounded-full"
        />
        <span className="text-2xl font-bold text-primary">Gaggle</span>
        <span
          className="h-8 w-8 animate-spin rounded-full border-2 border-primary border-t-transparent"
          aria-hidden="true"
        />
        {caption && <p className="text-sm text-muted-foreground">{caption}</p>}
      </div>
    </div>
  );
}