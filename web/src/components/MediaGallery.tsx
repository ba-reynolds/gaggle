import React, { useEffect, useState } from "react";
import * as DialogPrimitive from "@radix-ui/react-dialog";
import { Button } from "@/components/ui/button";
import { ChevronLeft, ChevronRight, X, MessageSquare, PenLine } from "lucide-react";
import { cn } from "@/lib/utils";

export interface GalleryItem {
  id: string;
  url: string;
  altText: string;
}

interface MediaGalleryProps {
  items: GalleryItem[];
  editable: boolean;
  preventImageViewer?: boolean;
  onRemove?: (id: string) => void;
  onEditAltText?: (item: GalleryItem) => void;
}

// Forward ref for DialogContent
type DialogContentElement = React.ComponentRef<typeof DialogPrimitive.Content>;
type DialogContentProps = React.ComponentPropsWithoutRef<typeof DialogPrimitive.Content>;

const FullscreenDialogContent = React.forwardRef<DialogContentElement, DialogContentProps>(
  ({ className, children, ...props }, ref) => (
    <DialogPrimitive.Portal>
      <DialogPrimitive.Overlay className="fixed inset-0 z-50 bg-black/20 backdrop-blur-[2px] data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0" />
      <DialogPrimitive.Content
        ref={ref}
        className={cn(
          "fixed inset-0 z-50 flex items-center justify-center p-4 duration-400 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0",
          className
        )}
        {...props}
      >
        {children}
      </DialogPrimitive.Content>
    </DialogPrimitive.Portal>
  )
);
FullscreenDialogContent.displayName = "FullscreenDialogContent";

// ImageViewer Component (previously separate file)
interface ImageViewerProps {
  isOpen: boolean;
  onClose: () => void;
  images: GalleryItem[];
  initialIndex: number;
}

const ImageViewer: React.FC<ImageViewerProps> = ({
  isOpen,
  onClose,
  images,
  initialIndex = 0,
}) => {
  const [currentIndex, setCurrentIndex] = useState(initialIndex);
  const [showAltText, setShowAltText] = useState(false);
  const currentImage = images[currentIndex];

  const handlePrevious = (e?: React.MouseEvent) => {
    e?.stopPropagation();
    if (currentIndex > 0) {
      setCurrentIndex(currentIndex - 1);
    }
  };

  const handleNext = (e?: React.MouseEvent) => {
    e?.stopPropagation();
    if (currentIndex < images.length - 1) {
      setCurrentIndex(currentIndex + 1);
    }
  };

  const handleOutsideClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (e.target === e.currentTarget) {
      onClose();
    }
  }

  // Handle keyboard navigation
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (!isOpen) return;
      
      switch (e.key) {
        case "ArrowLeft":
          handlePrevious();
          break;
        case "ArrowRight":
          handleNext();
          break;
        case "Escape":
          onClose();
          break;
        default:
          break;
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [isOpen, onClose, currentIndex, images.length]);

  // Reset to initial index when opening
  useEffect(() => {
    if (isOpen) {
      setCurrentIndex(initialIndex);
      setShowAltText(false);
    }
  }, [isOpen, initialIndex]);

  if (!isOpen || !images.length) return null;

  return (
    <DialogPrimitive.Root open={isOpen} onOpenChange={onClose}>
      <FullscreenDialogContent className="p-0">
        {/* Close button */}
        <Button
          variant="ghost"
          size="icon"
          className="absolute top-4 right-4 z-50 bg-black/10 text-white hover:bg-black/70 rounded-full"
          onClick={(e) => {
            e.stopPropagation();
            onClose();
          }}
        >
          <X className="h-5 w-5" />
        </Button>

        {/* Image container */}
        <div className="relative w-full h-full flex items-center justify-center" onClick={handleOutsideClick}>
          <img
            src={currentImage.url}
            alt={currentImage.altText || "Image"}
            className="max-h-[80%] max-w-[80%] object-contain rounded-md"
          />
          
          {/* Alt text toggle button - only show if there's alt text */}
          {currentImage.altText && (
            <Button
              variant="ghost"
              size="icon"
              className="absolute bottom-4 left-4 z-50 bg-black/50 text-white hover:bg-black/70 rounded-full h-10 w-10"
              onClick={(e) => {
                e.stopPropagation();
                setShowAltText(!showAltText);
              }}
            >
              <MessageSquare className="h-5 w-5" />
            </Button>
          )}
          
          {/* Alt text display */}
          {currentImage.altText && showAltText && (
            <div className="absolute bottom-16 left-4 bg-black/70 text-white px-4 py-2 rounded-md max-w-80 max-h-40 overflow-y-auto break-words whitespace-normal">
              {currentImage.altText}
            </div>
          )}

          {/* Navigation buttons - only show if there are multiple images */}
          {images.length > 1 && (
            <>
              {currentIndex > 0 && (
                <Button
                  variant="ghost"
                  size="icon"
                  className="absolute left-4 z-50 bg-black/50 text-white hover:bg-black/70 rounded-full h-12 w-12"
                  onClick={handlePrevious}
                >
                  <ChevronLeft className="h-6 w-6" />
                </Button>
              )}
              
              {currentIndex < images.length - 1 && (
                <Button
                  variant="ghost"
                  size="icon"
                  className="absolute right-4 z-50 bg-black/50 text-white hover:bg-black/70 rounded-full h-12 w-12"
                  onClick={handleNext}
                >
                  <ChevronRight className="h-6 w-6" />
                </Button>
              )}

              {/* Image counter */}
              <div className="absolute top-4 left-4 bg-black/70 text-white px-3 py-1 rounded-md">
                {currentIndex + 1} / {images.length}
              </div>
            </>
          )}
        </div>
      </FullscreenDialogContent>
    </DialogPrimitive.Root>
  );
};


const SingleImage: React.FC<{
  item: GalleryItem;
  clickable?: boolean;
  editable?: boolean;
  onRemove?: (id: string) => void;
  onEditAltText?: (item: GalleryItem) => void;
  heightClass?: string;
  className?: string;
  onClick?: (e: React.MouseEvent) => void;
}> = ({ item, clickable = false, editable = false, onRemove, onEditAltText, heightClass = "h-40", className = "", onClick }) => {
  return (
    <div className={`relative ${className}`}>
      <img 
        src={item.url} 
        alt={item.altText || "Media content"} 
        className={`w-full object-cover ${heightClass} ${clickable ? "cursor-pointer" : ""}`}
        onClick={onClick}
        onAuxClick={(e) => e.stopPropagation()}
        onMouseDown={(e) => { if (e.button === 1) e.stopPropagation(); }}
      />
      
      {editable && (
        <div className="absolute top-2 right-2 flex gap-2">
          {onEditAltText && (
            <Button
              variant="secondary"
              size="icon"
              className="rounded-full bg-foreground/50 text-primary-foreground hover:bg-foreground/70"
              onClick={(e) => {
                e.stopPropagation();
                onEditAltText(item);
              }}
            >
              <PenLine className="h-4 w-4" />
            </Button>
          )}
          {onRemove && (
            <Button
              variant="secondary"
              size="icon"
              className="rounded-full bg-foreground/50 text-primary-foreground hover:bg-foreground/70"
              onClick={(e) => {
                e.stopPropagation();
                onRemove(item.id);
              }}
            >
              <X className="h-4 w-4" />
            </Button>
          )}
        </div>
      )}
      {item.altText && (
        <div
          className="absolute bottom-2 left-2 bg-foreground/50 text-primary-foreground text-xs px-2 py-1 rounded-full"
          onClick={onClick}
        >
          ALT
        </div>
      )}
    </div>
  );
};

// Main MediaGallery Component
const MediaGallery: React.FC<MediaGalleryProps> = ({
  items,
  editable = false,
  onRemove,
  onEditAltText,
  preventImageViewer = false
}) => {
  const [viewerOpen, setViewerOpen] = useState(false);
  const [selectedImageIndex, setSelectedImageIndex] = useState(0);

  if (!items || items.length === 0) return null;

  const handleImageClick = (e: React.MouseEvent, index: number) => {
    e.stopPropagation(); // Prevent parent click events
    if (preventImageViewer) return;
    
    setSelectedImageIndex(index);
    setViewerOpen(true);
  };

  const h80 = "h-80";
  const h40 = "h-40";
  const clickable = !preventImageViewer;

  if (items.length === 1) {
    return (
      <>
        <div className="border border-border rounded-lg overflow-hidden">
          <SingleImage 
            item={items[0]} 
            clickable={clickable}
            editable={editable}
            onRemove={onRemove}
            onEditAltText={onEditAltText}
            heightClass={h80}
            onClick={(e) => handleImageClick(e, 0)}
          />
        </div>
        <ImageViewer 
          isOpen={viewerOpen}
          onClose={() => setViewerOpen(false)}
          images={items}
          initialIndex={selectedImageIndex}
        />
      </>
    );
  }

  if (items.length === 2) {
    return (
      <>
        <div className="flex rounded-lg overflow-hidden border border-border">
          {items.map((item, idx) => (
            <div key={item.id} className={`w-1/2 ${idx === 0 ? "border-r border-border" : ""}`}>
              <SingleImage 
                item={item}
                clickable={clickable}
                editable={editable}
                onRemove={onRemove}
                onEditAltText={onEditAltText}
                heightClass={h80}
                onClick={(e) => handleImageClick(e, idx)}
              />
            </div>
          ))}
        </div>
        <ImageViewer 
          isOpen={viewerOpen}
          onClose={() => setViewerOpen(false)}
          images={items}
          initialIndex={selectedImageIndex}
        />
      </>
    );
  }

  if (items.length === 3) {
    return (
      <>
        <div className="flex rounded-lg overflow-hidden border border-border">
          <div className="w-1/2 border-r border-border">
            <SingleImage 
              item={items[0]} 
              clickable={clickable}
              editable={editable}
              onRemove={onRemove}
              onEditAltText={onEditAltText}
              heightClass={h80}
              onClick={(e) => handleImageClick(e, 0)}
            />
          </div>
          <div className="w-1/2 flex flex-col">
            <div className="h-1/2 border-b border-border">
              <SingleImage 
                item={items[1]} 
                clickable={clickable}
                editable={editable}
                onRemove={onRemove}
                onEditAltText={onEditAltText}
                heightClass={h40}
                onClick={(e) => handleImageClick(e, 1)}
              />
            </div>
            <div className="h-1/2">
              <SingleImage 
                item={items[2]} 
                clickable={clickable}
                editable={editable}
                onRemove={onRemove}
                onEditAltText={onEditAltText}
                heightClass={h40}
                onClick={(e) => handleImageClick(e, 2)}
              />
            </div>
          </div>
        </div>
        <ImageViewer 
          isOpen={viewerOpen}
          onClose={() => setViewerOpen(false)}
          images={items}
          initialIndex={selectedImageIndex}
        />
      </>
    );
  }

  // 4+ images — grid of 2x2
  return (
    <>
      <div className="flex flex-wrap rounded-lg overflow-hidden border border-border">
        {items.slice(0, 4).map((item, idx) => (
          <div
            key={item.id}
            className={`w-1/2 ${idx < 2 ? "border-b border-border" : ""} ${
              idx % 2 === 0 ? "border-r border-border" : ""
            }`}
          >
            <SingleImage 
              item={item}
              clickable={clickable}
              editable={editable}
              onRemove={onRemove}
              onEditAltText={onEditAltText}
              heightClass={h40}
              onClick={(e) => handleImageClick(e, idx)}
            />
          </div>
        ))}
      </div>
      <ImageViewer 
        isOpen={viewerOpen}
        onClose={() => setViewerOpen(false)}
        images={items}
        initialIndex={selectedImageIndex}
      />
    </>
  );
};

export default MediaGallery;