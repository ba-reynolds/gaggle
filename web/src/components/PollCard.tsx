import { useVotePoll } from '@/hooks/usePost';
import type { Poll } from '@/types/api';
import { cn } from '@/lib/utils';
import { toast } from 'sonner';

export default function PollCard({ poll, postId }: { poll: Poll; postId: number }) {
  const vote = useVotePoll();
  const voted = poll.selected_option_id != null;
  return (
    <div className="mt-3 space-y-2 rounded-xl border border-border p-3" onClick={(event) => event.stopPropagation()}>
      <div className="flex items-center justify-between gap-2">
        <p className="font-medium text-primary">{poll.question}</p>
        <span className="text-xs text-muted-foreground">{poll.total_votes} votes</span>
      </div>
      {poll.options.map((option) => {
        const percentage = poll.total_votes ? Math.round((option.vote_count / poll.total_votes) * 100) : 0;
        return (
          <button
            key={option.id}
            disabled={voted || poll.closed || vote.isPending}
            onClick={() => vote.mutate({ postId, optionId: option.id }, { onError: () => toast.error('Could not record your vote') })}
            className={cn('relative w-full overflow-hidden rounded-lg border border-border p-3 text-left text-sm', !voted && !poll.closed && 'hover:border-primary', poll.selected_option_id === option.id && 'border-primary')}
          >
            {(voted || poll.closed) && <span className="absolute inset-y-0 left-0 bg-primary/10" style={{ width: `${percentage}%` }} />}
            <span className="relative flex justify-between gap-2"><span>{option.label}</span>{(voted || poll.closed) && <span>{percentage}%</span>}</span>
          </button>
        );
      })}
      {poll.closed && <p className="text-xs text-muted-foreground">Poll closed</p>}
    </div>
  );
}
