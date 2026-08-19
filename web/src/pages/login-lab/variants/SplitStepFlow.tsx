import { ArrowLeft, ArrowRight, Check, Loader2, Lock, Mail, User } from 'lucide-react';
import { useState, type FormEvent } from 'react';
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useLoginFlow } from '@/hooks/useLoginFlow';

type Step = 'identifier' | 'password';

export function SplitStepFlow() {
  const { form, loginMutation, onSubmit } = useLoginFlow();
  const [step, setStep] = useState<Step>('identifier');
  const identifier = form.watch('identifier');

  const advanceToPassword = async () => {
    const identifierValid = await form.trigger('identifier');
    if (identifierValid) {
      setStep('password');
    }
  };

  const onFormSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (step === 'identifier') {
      await advanceToPassword();
    } else {
      await form.handleSubmit(onSubmit)();
    }
  };

  return (
    <div className="flex h-full w-full">
      <div className="hidden flex-1 flex-col justify-between bg-gradient-to-br from-primary via-primary to-chart-1 p-12 text-primary-foreground lg:flex">
        <div className="flex items-center gap-3">
          <span className="flex h-10 w-10 items-center justify-center rounded-full bg-background/20 text-xl font-bold">
            G
          </span>
          <span className="text-lg font-semibold tracking-tight">Gaggle</span>
        </div>
        <div className="max-w-md space-y-6">
          <h2 className="text-4xl font-bold leading-tight tracking-tight">
            A social feed that feels like home.
          </h2>
          <p className="text-primary-foreground/80">Fewer fields. Faster flock.</p>
        </div>
        <ul className="space-y-3 text-sm text-primary-foreground/90">
          <li className="flex items-center gap-2">
            <ArrowRight size={16} />
            One field at a time, no overwhelm
          </li>
          <li className="flex items-center gap-2">
            <ArrowRight size={16} />
            Real conversations, zero noise
          </li>
          <li className="flex items-center gap-2">
            <ArrowRight size={16} />
            Honk responsibly
          </li>
        </ul>
      </div>

      <div className="flex flex-1 items-center justify-center p-8">
        <div className="w-full max-w-sm space-y-8">
          <div className="lg:hidden">
            <span className="flex h-12 w-12 items-center justify-center rounded-full bg-primary text-xl font-bold text-primary-foreground">
              G
            </span>
          </div>

          <div className="flex items-center justify-end gap-2">
            {(['identifier', 'password'] as Step[]).map((s) => (
              <span
                key={s}
                className={`h-1.5 rounded-full transition-all duration-300 ${
                  step === s ? 'w-8 bg-primary' : 'w-3 bg-muted'
                }`}
              />
            ))}
          </div>

          <Form {...form}>
            <form onSubmit={onFormSubmit} className="space-y-4">
              {step === 'identifier' ? (
                <div
                  key="identifier"
                  className="animate-in slide-in-from-bottom-2 duration-300 space-y-4"
                >
                  <div className="space-y-1">
                    <h1 className="text-2xl font-bold tracking-tight">Welcome back</h1>
                    <p className="text-sm text-muted-foreground">
                      What&apos;s your username or email?
                    </p>
                  </div>

                  <FormField
                    control={form.control}
                    name="identifier"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Username or Email</FormLabel>
                        <FormControl>
                          <div className="relative">
                            <div className="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none text-muted-foreground">
                              {field.value.includes('@') ? <Mail size={18} /> : <User size={18} />}
                            </div>
                            <Input
                              placeholder="you@example.com"
                              className="h-12 pl-10"
                              autoFocus
                              {...field}
                            />
                          </div>
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <Button type="submit" className="h-12 w-full" disabled={!identifier}>
                    Continue
                    <ArrowRight size={18} />
                  </Button>
                </div>
              ) : (
                <div
                  key="password"
                  className="animate-in slide-in-from-bottom-2 duration-300 space-y-4"
                >
                  <div className="space-y-1">
                    <h1 className="text-2xl font-bold tracking-tight">Welcome back</h1>
                    <p className="flex items-center gap-2 text-sm text-muted-foreground">
                      Signing in as
                      <span className="rounded-full bg-muted px-2 py-0.5 text-xs text-foreground">
                        {identifier}
                      </span>
                    </p>
                  </div>

                  <FormField
                    control={form.control}
                    name="password"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Password</FormLabel>
                        <FormControl>
                          <div className="relative">
                            <div className="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none text-muted-foreground">
                              <Lock size={18} />
                            </div>
                            <Input
                              type="password"
                              placeholder="Enter your password"
                              className="h-12 pl-10"
                              autoFocus
                              {...field}
                            />
                          </div>
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <div className="flex items-center gap-2">
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="h-12 w-12"
                      aria-label="Back"
                      onClick={() => setStep('identifier')}
                    >
                      <ArrowLeft size={18} />
                    </Button>
                    <Button
                      type="submit"
                      className="h-12 flex-1"
                      disabled={loginMutation.isPending}
                    >
                      {loginMutation.isPending ? (
                        <Loader2 size={18} className="animate-spin" />
                      ) : (
                        <Check size={18} />
                      )}
                      {loginMutation.isPending ? 'Signing in...' : 'Sign in'}
                    </Button>
                  </div>
                </div>
              )}
            </form>
          </Form>
        </div>
      </div>
    </div>
  );
}