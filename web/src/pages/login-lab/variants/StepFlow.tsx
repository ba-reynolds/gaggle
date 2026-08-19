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
import { ArrowLeft, ArrowRight, Check, Loader2, Lock, Mail, User } from 'lucide-react';
import { useState, type FormEvent, type ReactNode } from 'react';

type Step = 'identifier' | 'password';

export function StepFlow({ footer }: { footer?: ReactNode } = {}) {
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
    <div className="flex h-full w-full items-center justify-center p-8">
      <div className="w-full max-w-sm space-y-8">
        <div className="flex items-center justify-center">
          <span className="flex h-14 w-14 items-center justify-center rounded-2xl bg-primary text-xl font-bold text-primary-foreground">
            G
          </span>
        </div>

        <div className="flex items-center justify-center gap-2">
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
          <form
            onSubmit={onFormSubmit}
            className="space-y-6"
          >
            {step === 'identifier' ? (
              <div key="identifier" className="animate-in slide-in-from-bottom-2 duration-300 space-y-4">
                <FormField
                  control={form.control}
                  name="identifier"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel className="text-base font-semibold">
                        What&apos;s your username or email?
                      </FormLabel>
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
                <Button
                  type="submit"
                  className="h-12 w-full"
                  disabled={!identifier}
                >
                  Continue
                  <ArrowRight size={18} />
                </Button>
              </div>
            ) : (
              <div key="password" className="animate-in slide-in-from-bottom-2 duration-300 space-y-4">
                <FormField
                  control={form.control}
                  name="password"
                  render={({ field }) => (
                    <FormItem>
                      <h2 className="flex items-center gap-2 text-base font-semibold">
                        Welcome back
                        <span className="rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground">
                          {identifier}
                        </span>
                      </h2>
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

        {footer}
      </div>
    </div>
  );
}