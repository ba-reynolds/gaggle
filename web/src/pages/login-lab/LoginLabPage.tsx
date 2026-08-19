import { FlaskConical } from 'lucide-react';
import { Link, useSearchParams } from 'react-router-dom';
import { cn } from '@/lib/utils';
import { loginVariants } from './variants';

export function LoginLabPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const selectedId = searchParams.get('v') ?? loginVariants[0].id;
  const selected =
    loginVariants.find((v) => v.id === selectedId) ?? loginVariants[0];
  const VariantComponent = selected.Component;

  return (
    <div className="flex h-screen overflow-hidden bg-background text-foreground">
      <aside className="flex w-72 shrink-0 flex-col border-r bg-card">
        <div className="flex items-center gap-2 border-b px-4 py-3">
          <FlaskConical size={18} className="text-muted-foreground" />
          <div>
            <h1 className="text-sm font-semibold leading-none">Login lab</h1>
            <p className="mt-1 text-xs text-muted-foreground">
              Design experiments — pick one to keep
            </p>
          </div>
        </div>

        <nav className="flex-1 overflow-y-auto p-2">
          {(['Style', 'Flow'] as const).map((category) => (
            <div key={category} className="mb-2">
              <p className="px-3 py-2 text-xs font-medium uppercase tracking-wider text-muted-foreground">
                {category}
              </p>
              <ul className="space-y-1">
                {loginVariants
                  .filter((v) => v.Category === category)
                  .map((variant) => (
                    <li key={variant.id}>
                      <button
                        type="button"
                        onClick={() => setSearchParams({ v: variant.id })}
                        className={cn(
                          'w-full rounded-lg px-3 py-2 text-left transition-colors',
                          variant.id === selected.id
                            ? 'bg-primary text-primary-foreground'
                            : 'hover:bg-accent hover:text-accent-foreground',
                        )}
                      >
                        <span className="block text-sm font-medium">{variant.name}</span>
                        <span
                          className={cn(
                            'block text-xs',
                            variant.id === selected.id
                              ? 'text-primary-foreground/70'
                              : 'text-muted-foreground',
                          )}
                        >
                          {variant.description}
                        </span>
                      </button>
                    </li>
                  ))}
              </ul>
            </div>
          ))}
        </nav>

        <div className="border-t p-3 text-center">
          <Link
            to="/login"
            className="text-xs text-muted-foreground hover:text-foreground"
          >
            ← Back to the current login page
          </Link>
        </div>
      </aside>

      <main className="h-full flex-1 overflow-hidden">
        <VariantComponent />
      </main>
    </div>
  );
}