import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { useMyLists, useCreateList, useDeleteList, useUpdateList, useAddUsersToList, useListMembers } from '@/hooks/useLists';
import { useFetchUserFollowing } from '@/hooks/useUser';
import { useUser } from '@/contexts/UserContext';
import { Button } from '@/components/ui/button';
import { Dialog, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog';
import { CustomDialogContent } from '@/components/ui/custom-dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { List as ListIcon, Loader2, Pencil, Plus, Trash2, UserPlus, Users } from 'lucide-react';
import { getMediaUrl } from '@/util/media';
import { toast } from 'sonner';
import ConfirmDialog from '@/components/ConfirmDialog';
import type { List } from '@/types/api';

interface DialogState {
  mode: 'create' | 'edit';
  list?: List;
}

const ListsPage: React.FC = () => {
  const { user } = useUser();
  const { data, isLoading } = useMyLists();
  const createList = useCreateList();
  const deleteList = useDeleteList();
  const updateList = useUpdateList();
  const addMembers = useAddUsersToList();
  const lists = data?.data ?? [];

  const [dialog, setDialog] = useState<DialogState | null>(null);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [toAdd, setToAdd] = useState<string[]>([]);
  const [deleteTarget, setDeleteTarget] = useState<number | null>(null);

  const { data: followingData } = useFetchUserFollowing(user.username);
  const following = followingData?.data?.items ?? [];

  const editingListId = dialog?.mode === 'edit' ? dialog.list?.id ?? 0 : 0;
  const { data: membersData } = useListMembers(editingListId, 200);
  const existingMembers = new Set((membersData?.pages ?? []).flatMap((p) => p.data.items).map((m) => m.username));

  useEffect(() => {
    if (dialog?.mode === 'edit' && dialog.list) {
      setName(dialog.list.name);
      setDescription(dialog.list.description ?? '');
    } else if (dialog?.mode === 'create') {
      setName('');
      setDescription('');
    }
    setToAdd([]);
  }, [dialog]);

  const openCreate = () => setDialog({ mode: 'create' });
  const openEdit = (list: List) => setDialog({ mode: 'edit', list });

  const toggleSuggested = (username: string) => {
    setToAdd((prev) => (prev.includes(username) ? prev.filter((u) => u !== username) : [...prev, username]));
  };

  const submit = () => {
    if (!name.trim()) return;
    const payload = { name: name.trim(), description: description.trim() };
    const finish = (listId: number) => {
      if (toAdd.length > 0) {
        addMembers.mutate(
          { listId, usernames: toAdd },
          { onError: () => toast.error('List saved, but some members could not be added') }
        );
      }
      setDialog(null);
    };
    if (dialog?.mode === 'edit' && dialog.list) {
      updateList.mutate({ listId: dialog.list.id, payload }, {
        onSuccess: (res) => { toast.success('List updated'); finish(res.data.id); },
        onError: () => toast.error('Failed to update list'),
      });
    } else {
      createList.mutate(payload, {
        onSuccess: (res) => { toast.success('List created'); finish(res.data.id); },
        onError: () => toast.error('Failed to create list'),
      });
    }
  };

  const remove = (id: number) => {
    deleteList.mutate(id, {
      onSuccess: () => toast.success('List deleted'),
      onError: () => toast.error('Failed to delete list'),
    });
  };

  const savePending = createList.isPending || updateList.isPending || addMembers.isPending;
  const suggestions = following.filter((f) => !existingMembers.has(f.username));

  return (
    <div className="mx-auto w-full max-w-xl pt-6">
      <header className="px-4 pb-4 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-primary">Lists</h1>
          <p className="text-sm text-muted-foreground">Collections of accounts, curated by you.</p>
        </div>
        <Button onClick={openCreate}><Plus className="h-4 w-4 mr-2" />New list</Button>
      </header>

      {isLoading ? (
        <div className="flex justify-center py-12"><Loader2 className="h-6 w-6 animate-spin" /></div>
      ) : lists.length === 0 ? (
        <div className="flex flex-col items-center gap-3 p-12 text-center text-muted-foreground">
          <ListIcon className="h-10 w-10" />
          <p>You don't have any lists yet.</p>
        </div>
      ) : (
        <div className="space-y-2 px-4 pb-8">
          {lists.map((list) => (
            <div key={list.id} className="flex items-center justify-between rounded-xl border border-border p-4 hover:bg-muted">
              <Link to={`/lists/${list.id}`} className="min-w-0 flex-1">
                <p className="font-semibold text-primary">{list.name}</p>
                <p className="text-sm text-muted-foreground truncate">{list.description || 'No description'}</p>
                <p className="text-xs text-muted-foreground mt-1 flex items-center gap-1">
                  <Users className="h-3 w-3" /> {list.member_count} members
                </p>
              </Link>
              <div className="flex items-center gap-1">
                <Button variant="ghost" size="icon" onClick={() => openEdit(list)} title="Edit list">
                  <Pencil className="h-4 w-4" />
                </Button>
                <Button variant="ghost" size="icon" className="text-destructive" onClick={() => setDeleteTarget(list.id)}>
                  <Trash2 className="h-4 w-4" />
                </Button>
              </div>
            </div>
          ))}
        </div>
      )}

      <Dialog open={dialog !== null} onOpenChange={(o) => !o && setDialog(null)}>
        <CustomDialogContent className="sm:max-w-md bg-card">
          <DialogHeader><DialogTitle className="text-primary">{dialog?.mode === 'edit' ? 'Edit list' : 'New list'}</DialogTitle></DialogHeader>
          <div className="space-y-3">
            <div className="space-y-2">
              <Label htmlFor="list-name">Name</Label>
              <Input id="list-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="Go developers" maxLength={100} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="list-desc">Description</Label>
              <Textarea id="list-desc" value={description} onChange={(e) => setDescription(e.target.value)} placeholder="What's this list about?" maxLength={300} className="min-h-20" />
            </div>

            {suggestions.length > 0 && (
              <div className="space-y-2">
                <Label>Suggestions — people you follow</Label>
                <div className="max-h-48 overflow-y-auto space-y-1 rounded-lg border border-border p-2">
                  {suggestions.map((s) => {
                    const selected = toAdd.includes(s.username);
                    const inList = existingMembers.has(s.username);
                    return (
                      <button
                        key={s.username}
                        type="button"
                        disabled={inList}
                        onClick={() => toggleSuggested(s.username)}
                        className={`flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm transition-colors ${selected ? 'bg-primary/10' : 'hover:bg-muted'} ${inList ? 'opacity-50' : ''}`}
                      >
                        <Avatar className="h-6 w-6">
                          <AvatarImage src={getMediaUrl(s.profile_picture_uuid)} alt={s.display_name} />
                          <AvatarFallback>{s.display_name.charAt(0)}</AvatarFallback>
                        </Avatar>
                        <span className="min-w-0 flex-1 truncate">
                          <span className="font-medium text-primary">{s.display_name}</span>
                          <span className="ml-1 text-xs text-muted-foreground">@{s.username}</span>
                        </span>
                        {inList ? (
                          <span className="text-xs text-muted-foreground">In list</span>
                        ) : selected ? (
                          <span className="text-xs text-primary">Will add</span>
                        ) : (
                          <UserPlus className="h-4 w-4 text-muted-foreground" />
                        )}
                      </button>
                    );
                  })}
                </div>
              </div>
            )}
            {following.length === 0 && (
              <p className="text-xs text-muted-foreground">Follow people to get suggestions for adding them to this list.</p>
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialog(null)}>Cancel</Button>
            <Button disabled={!name.trim() || savePending} onClick={submit}>
              {savePending && <Loader2 className="h-4 w-4 animate-spin mr-2" />}
              {dialog?.mode === 'edit' ? 'Save' : 'Create'}
            </Button>
          </DialogFooter>
        </CustomDialogContent>
      </Dialog>

      <ConfirmDialog
        open={deleteTarget !== null}
        title="Delete this list?"
        description="This will permanently remove the list and its members."
        confirmLabel="Delete"
        onConfirm={() => deleteTarget !== null && remove(deleteTarget)}
        onOpenChange={(o) => !o && setDeleteTarget(null)}
      />
    </div>
  );
};

export default ListsPage;
