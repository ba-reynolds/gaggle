import React, { useState, useRef, useEffect } from "react";
import type { ChangeEvent } from "react";
import { Button } from "@/components/ui/button";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Textarea } from "@/components/ui/textarea";
import { Input } from "@/components/ui/input";
import { Image as ImageIcon, Globe, Users, Loader2, BarChart3, Plus, X } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  Dialog,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { useUser } from "@/contexts/UserContext";
import MediaGallery, { type GalleryItem } from "./MediaGallery";
import { useMediaUpload } from "@/hooks/useMediaUpload";
import { useCreatePost } from "@/hooks/usePost";
import { CustomDialogContent } from "./ui/custom-dialog";
import { getMediaUrl } from "@/util/media";
import type { CreatePollPayload, MediaItem } from "@/types/api";
import { toast } from "sonner";
import { useNavigate } from "react-router-dom";

type Visibility = "Everyone" | "Followers";

export interface PostData {
  text: string;
  media: GalleryItem[];
  visibility: Visibility;
  poll?: CreatePollPayload;
}

export interface ComposeContentProps {
  onSubmit?: (data: PostData) => void;
  onError?: () => void;
  placeholder?: string;
  submitLabel?: string;
  textareaHeight?: string;
  children?: React.ReactNode;
  handlePostCreation?: boolean;
  parentId?: number | null;
}

const ComposeContent: React.FC<ComposeContentProps> = ({
  onSubmit,
  onError,
  placeholder = "What's happening?",
  submitLabel = "Post",
  textareaHeight = "h-32",
  children,
  handlePostCreation = true,
  parentId = null
}) => {
  const { user } = useUser();
  const navigate = useNavigate();
  const [text, setText] = useState("");
  const [mediaItems, setMediaItems] = useState<GalleryItem[]>([]);
  const [mediaFiles, setMediaFiles] = useState<File[]>([]);
  const [visibility, setVisibility] = useState<Visibility>("Everyone");
  const [pollEnabled, setPollEnabled] = useState(false);
  const [pollQuestion, setPollQuestion] = useState("");
  const [pollOptions, setPollOptions] = useState(["", ""]);
  const [isUploading, setIsUploading] = useState(false);
  const [currentEditingMedia, setCurrentEditingMedia] = useState<GalleryItem | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  
  // Use the media upload mutation
  const mediaUploadMutation = useMediaUpload();
  
  // Use the create post mutation
  const createPostMutation = useCreatePost();

  const handleSubmit = async () => {
    try {
      let mediaPayload: MediaItem[] = [];

      // If we have media files, upload them first
      if (mediaFiles.length > 0) {
        const mediaUploadResult = await mediaUploadMutation.mutateAsync(mediaFiles);

        if (mediaUploadResult.data.uuids) {
          // Map the UUIDs to the media items to include alt text
          mediaPayload = mediaUploadResult.data.uuids.map((uuid, index) => ({
            uuid,
            alt_text: mediaItems[index].altText
          }));
        }
      }

      const poll = pollEnabled ? { question: pollQuestion, options: pollOptions.filter(Boolean) } : undefined;
      if (handlePostCreation) {
        // Create the post with the media UUIDs
        const response = await createPostMutation.mutateAsync({
          content: text,
          media: mediaPayload,
          parent_id: parentId,
          poll,
        });

        // Show success toast with link to the new post
        toast.success(
          <div className="flex flex-col gap-1">
            <p>Post created successfully!</p>
            <a 
              href={`/post/${response.data.id}`}
              onClick={(e) => {
                e.preventDefault();
                navigate(`/post/${response.data.id}`);
              }}
              className="text-blue-500 hover:underline"
            >
              View your post
            </a>
          </div>
        );
      }

      // Call the onSubmit callback with the local data if provided
      if (onSubmit) {
        onSubmit({
          text,
          media: mediaItems,
          visibility,
          poll,
        });
      }

      // Reset form state
      setText("");
      setMediaItems([]);
      setMediaFiles([]);
      setVisibility("Everyone");
      setPollEnabled(false);
      setPollQuestion("");
      setPollOptions(["", ""]);
    } catch {
      if (onError) {
        onError();
      } else {
        toast.error("Failed to create post. Please try again.");
      }
    }
  };

  const handleFileSelect = (e: ChangeEvent<HTMLInputElement>) => {
    if (!e.target.files || e.target.files.length === 0) return;
    
    // Check if adding more files would exceed the limit
    if (mediaItems.length + e.target.files.length > 4) {
      alert("You can only upload up to 4 images or GIFs");
      return;
    }

    setIsUploading(true);
    
    // Store the actual File objects for later upload
    const newFiles = Array.from(e.target.files);
    setMediaFiles(prev => [...prev, ...newFiles]);
    
    // Process each file for preview
    newFiles.forEach(file => {
      const fileUrl = URL.createObjectURL(file);
      
      const newMediaItem: GalleryItem = {
        id: `media-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
        url: fileUrl,
        altText: ""
      };
      
      setMediaItems(prev => [...prev, newMediaItem]);
    });
    
    // Reset the file input
    if (fileInputRef.current) {
      fileInputRef.current.value = "";
    }
    
    setIsUploading(false);
  };

  const removeMediaItem = (id: string) => {
    // Find the index of the media item to remove
    const index = mediaItems.findIndex(item => item.id === id);
    URL.revokeObjectURL(mediaItems[index].url);

    if (index !== -1) {
      // Remove the media item
      setMediaItems(prev => prev.filter(item => item.id !== id));
      
      // Remove the corresponding file
      setMediaFiles(prev => {
        const newFiles = [...prev];
        newFiles.splice(index, 1);
        return newFiles;
      });
    }
  };

  const openAltTextDialog = (mediaItem: GalleryItem) => {
    setCurrentEditingMedia(mediaItem);
  };

  const saveAltText = (altText: string) => {
    if (!currentEditingMedia) return;
    
    setMediaItems(prev => 
      prev.map(item => 
        item.id === currentEditingMedia.id 
          ? { ...item, altText } 
          : item
      )
    );
    
    setCurrentEditingMedia(null);
  };

  const handleImageButtonClick = () => {
    if (mediaItems.length >= 4) {
      alert("You can only upload up to 4 images or GIFs");
      return;
    }
    fileInputRef.current?.click();
  };

  // Cleanup the media items when the component unmounts
  useEffect(() => {
    return () => {
      mediaItems.forEach(item => {
        URL.revokeObjectURL(item.url);
      });
    };
  }, [mediaItems]);

  // Determine if the submit button should be disabled
  const isSubmitDisabled = 
    (!text.trim() && mediaItems.length === 0 && !pollEnabled) || 
    mediaUploadMutation.isPending || 
    createPostMutation.isPending;

  return (
    <div className="flex space-x-3">
      <Avatar className="h-10 w-10">
        <AvatarImage src={getMediaUrl(user.profilePictureUUID)} />
        <AvatarFallback>{user.username[0]}</AvatarFallback>
      </Avatar>

      <div className="flex-1">
        {children}

        <Textarea
          placeholder={placeholder}
          className={`border-none resize-none focus-visible:ring-0 focus-visible:ring-offset-0 shadow-md ${textareaHeight} text-primary`}
          value={text}
          onChange={(e) => setText(e.target.value)}
        />

        {/* Hidden file input for image/gif selection */}
        <input 
          type="file" 
          ref={fileInputRef}
          className="hidden"
          accept="image/*,.gif" 
          multiple
          onChange={handleFileSelect}
        />

        {/* Media gallery */}
        {mediaItems.length > 0 && (
          <div className="mt-2">
            <MediaGallery 
              items={mediaItems} 
              editable={true}
              onRemove={removeMediaItem}
              onEditAltText={openAltTextDialog}
            />
          </div>
        )}

        <div className="flex justify-between mt-3">
          <div className="flex gap-2">
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="rounded-full text-primary hover:bg-primary/10"
                    onClick={handleImageButtonClick}
                    disabled={isUploading || mediaItems.length >= 4}
                  >
                    <ImageIcon className="h-5 w-5" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>
                  <p>Add images or GIFs (up to 4)</p>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>

            {!parentId && <Button variant="ghost" size="icon" className="rounded-full text-primary hover:bg-primary/10" onClick={() => setPollEnabled((enabled) => !enabled)}>
              <BarChart3 className="h-5 w-5" />
            </Button>}


            {/* Dropdown menu for visibility options ("who can see this") */}
            <DropdownMenu>
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <DropdownMenuTrigger asChild>
                      <Button variant="ghost" size="icon" className="rounded-full text-primary hover:bg-primary/10">
                        {visibility === "Everyone" ? (
                          <Globe className="h-5 w-5" />
                        ) : (
                          <Users className="h-5 w-5" />
                        )}
                      </Button>
                    </DropdownMenuTrigger>
                  </TooltipTrigger>
                  <TooltipContent>
                    <p>Choose who can see this</p>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>

              {/* Dropdown menu options */}
              <DropdownMenuContent align="start" className="w-56 border border-muted">
                <DropdownMenuItem
                  className={`cursor-pointer ${visibility === "Everyone" ? "bg-accent" : ""}`}
                  onClick={() => setVisibility("Everyone")}
                >
                  <Globe className="h-4 w-4 mr-2" />
                  <span>Everyone</span>
                </DropdownMenuItem>
                <DropdownMenuItem
                  className={`cursor-pointer ${visibility === "Followers" ? "bg-accent" : ""}`}
                  onClick={() => setVisibility("Followers")}
                >
                  <Users className="h-4 w-4 mr-2" />
                  <span>Followers only</span>
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>

          <Button
            onClick={handleSubmit}
            disabled={isSubmitDisabled}
            className="rounded-full font-medium bg-primary text-primary-foreground hover:bg-primary/90"
          >
            {mediaUploadMutation.isPending || createPostMutation.isPending ? (
              <Loader2 className="h-4 w-4 animate-spin mr-2" />
            ) : null}
            {submitLabel}
          </Button>
        </div>

        {pollEnabled && !parentId && <div className="mt-3 space-y-2 rounded-xl border border-border p-3">
          <Input placeholder="Poll question" value={pollQuestion} onChange={(event) => setPollQuestion(event.target.value)} maxLength={140} />
          {pollOptions.map((option, index) => <div key={index} className="flex gap-2">
            <Input placeholder={`Option ${index + 1}`} value={option} onChange={(event) => setPollOptions((current) => current.map((item, itemIndex) => itemIndex === index ? event.target.value : item))} maxLength={100} />
            {pollOptions.length > 2 && <Button type="button" variant="ghost" size="icon" onClick={() => setPollOptions((current) => current.filter((_, itemIndex) => itemIndex !== index))}><X className="h-4 w-4" /></Button>}
          </div>)}
          {pollOptions.length < 4 && <Button type="button" variant="ghost" size="sm" onClick={() => setPollOptions((current) => [...current, ""])}><Plus className="mr-1 h-4 w-4" /> Add option</Button>}
        </div>}
      </div>

      {/* Alt text dialog */}
      <Dialog open={!!currentEditingMedia} onOpenChange={(open) => !open && setCurrentEditingMedia(null)}>
        <CustomDialogContent className="sm:max-w-md max-h-[90vh] overflow-y-auto bg-card">
          <DialogHeader>
            <DialogTitle className="text-primary">Add alt text</DialogTitle>
          </DialogHeader>
          
          <div className="flex flex-col gap-4">
            <div className="flex justify-center">
              {currentEditingMedia && (
                <img 
                  src={currentEditingMedia.url} 
                  alt="Preview" 
                  className="max-h-48 object-contain rounded-md"
                />
              )}
            </div>
            
            <div className="space-y-2">
              <Label htmlFor="alt-text" className="text-primary">Alt text</Label>
              <Textarea
                id="alt-text"
                placeholder="Describe this image for people who can't see it..."
                value={currentEditingMedia?.altText || ""}
                onChange={(e) => {
                  if (currentEditingMedia && e.target.value.length <= 200) {
                    setCurrentEditingMedia({
                      ...currentEditingMedia,
                      altText: e.target.value
                    });
                  }
                }}
                className="resize-none h-32 bg-card text-primary border-border"
              />
              <div className="text-xs text-muted-foreground text-right">
                {currentEditingMedia?.altText.length || 0}/200
              </div>
            </div>
          </div>
          
          <DialogFooter>
            <Button 
              variant="outline" 
              onClick={() => setCurrentEditingMedia(null)}
              className="border-border text-primary"
            >
              Cancel
            </Button>
            <Button 
              onClick={() => saveAltText(currentEditingMedia?.altText || "")}
              className="bg-primary text-primary-foreground hover:bg-primary/90"
            >
              Save
            </Button>
          </DialogFooter>
        </CustomDialogContent>
      </Dialog>
    </div>
  );
};

export default ComposeContent;
