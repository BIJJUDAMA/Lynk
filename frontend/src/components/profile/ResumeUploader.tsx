"use client";

import React, { useState, useRef, useEffect } from "react";
import Link from "next/link";
import {
  FileText,
  Upload,
  Download,
  ShieldAlert,
  CheckCircle2,
  AlertCircle,
  Loader2,
  RefreshCw,
  ExternalLink,
} from "lucide-react";
import { useAuth } from "@/components/auth/AuthProvider";
import {
  uploadResume,
  presignResume,
  confirmResume,
  getMyResumeUrl,
  ApiClientError,
} from "@/lib/api";
import { formatFileSize, formatJobDate } from "@/lib/formatters";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import type { ResumeUploadResponse } from "@/types/api";

const MAX_FILE_SIZE_BYTES = 10 * 1024 * 1024; // 10MB

export interface ResumeUploaderProps {
  resumeKey?: string | null;
  resumeFilename?: string | null;
  resumeByteSize?: number;
  updatedAt?: string;
  isVerified?: boolean;
  onUploadSuccess?: (uploaded: ResumeUploadResponse) => void;
}

export function ResumeUploader({
  resumeKey: propResumeKey,
  resumeFilename: propResumeFilename,
  resumeByteSize: propResumeByteSize,
  updatedAt: propUpdatedAt,
  isVerified: propIsVerified,
  onUploadSuccess,
}: ResumeUploaderProps) {
  const { isVerified: authIsVerified } = useAuth();
  const isVerified = propIsVerified !== undefined ? propIsVerified : authIsVerified;

  // Local state for active resume
  const [currentResume, setCurrentResume] = useState<{
    key?: string | null;
    filename?: string | null;
    byteSize?: number;
    updatedAt?: string;
  }>({
    key: propResumeKey,
    filename: propResumeFilename,
    byteSize: propResumeByteSize,
    updatedAt: propUpdatedAt,
  });

  // Sync props if parent updates
  useEffect(() => {
    setCurrentResume({
      key: propResumeKey,
      filename: propResumeFilename,
      byteSize: propResumeByteSize,
      updatedAt: propUpdatedAt,
    });
  }, [propResumeKey, propResumeFilename, propResumeByteSize, propUpdatedAt]);

  const [isDragging, setIsDragging] = useState(false);
  const [isUploading, setIsUploading] = useState(false);
  const [isDownloading, setIsDownloading] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [showUploaderOverride, setShowUploaderOverride] = useState(false);

  const fileInputRef = useRef<HTMLInputElement | null>(null);

  const hasResume = Boolean(currentResume.key || currentResume.filename);

  const validateFile = (file: File): string | null => {
    if (file.size > MAX_FILE_SIZE_BYTES) {
      return "File size exceeds 10MB limit. Please upload a smaller PDF or DOCX file.";
    }
    if (file.size <= 0) {
      return "The selected file is empty. Please select a valid document.";
    }

    const lowerName = file.name.toLowerCase();
    const isPdf = lowerName.endsWith(".pdf") || file.type === "application/pdf";
    const isDocx =
      lowerName.endsWith(".docx") ||
      file.type === "application/vnd.openxmlformats-officedocument.wordprocessingml.document";

    if (!isPdf && !isDocx) {
      return "Invalid file format. Only PDF (.pdf) and Word (.docx) documents are accepted.";
    }

    return null;
  };

  const handleFileUpload = async (file: File) => {
    setErrorMessage(null);
    setSuccessMessage(null);

    // Strict Institutional Email Verification Gate
    if (!isVerified) {
      setErrorMessage(
        "Institutional .edu verification required. You must verify your university email before uploading your resume."
      );
      return;
    }

    // Client-side file validation
    const validationError = validateFile(file);
    if (validationError) {
      setErrorMessage(validationError);
      return;
    }

    setIsUploading(true);
    try {
      let uploadedKey = "";
      let uploadedFilename = file.name;
      let uploadedSize = file.size;

      // Attempt Direct-to-MinIO presigned PUT upload
      try {
        const presignRes = await presignResume({
          filename: file.name,
          content_type: file.type || "application/pdf",
          size: file.size,
        });

        const uploadRes = await fetch(presignRes.upload_url, {
          method: "PUT",
          body: file,
          headers: {
            "Content-Type": file.type || "application/pdf",
          },
        });

        if (!uploadRes.ok) {
          throw new Error(`Direct storage upload failed with status ${uploadRes.status}`);
        }

        const profile = await confirmResume({
          key: presignRes.key,
          filename: file.name,
          size: file.size,
        });

        uploadedKey = presignRes.key;
        if (profile.resume_filename) uploadedFilename = profile.resume_filename;
        if (profile.resume_byte_size) uploadedSize = profile.resume_byte_size;
      } catch {
        // Fallback to legacy multipart streaming upload if direct upload fails
        const legacyRes = await uploadResume(file);
        uploadedKey = legacyRes.resume_key;
        uploadedFilename = legacyRes.filename;
        uploadedSize = legacyRes.byte_size;
      }

      const uploadResult: ResumeUploadResponse = {
        resume_key: uploadedKey,
        filename: uploadedFilename,
        byte_size: uploadedSize,
        message: "Resume securely stored in campus vault.",
      };

      setCurrentResume({
        key: uploadedKey,
        filename: uploadedFilename,
        byteSize: uploadedSize,
        updatedAt: new Date().toISOString(),
      });
      setShowUploaderOverride(false);
      setSuccessMessage("Resume uploaded and secured in campus vault.");
      onUploadSuccess?.(uploadResult);
    } catch (err: unknown) {
      if (err instanceof ApiClientError) {
        if (err.isEmailNotVerified) {
          setErrorMessage(
            "Institutional .edu email verification required before uploading a resume."
          );
        } else if (err.code === "FILE_TOO_LARGE") {
          setErrorMessage("Resume file exceeds the 10MB size limit.");
        } else if (err.code === "INVALID_FILE_TYPE") {
          setErrorMessage("Resume must be a valid PDF or DOCX file.");
        } else {
          setErrorMessage(err.message || "Failed to upload resume.");
        }
      } else if (err instanceof Error) {
        setErrorMessage(err.message || "Failed to upload resume.");
      } else {
        setErrorMessage("An unexpected error occurred while uploading your resume.");
      }
    } finally {
      setIsUploading(false);
      if (fileInputRef.current) {
        fileInputRef.current.value = "";
      }
    }
  };

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      handleFileUpload(file);
    }
  };

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    if (!isVerified || isUploading) return;
    setIsDragging(true);
  };

  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
    if (!isVerified || isUploading) return;

    const file = e.dataTransfer.files?.[0];
    if (file) {
      handleFileUpload(file);
    }
  };

  const handleDownloadResume = async () => {
    setIsDownloading(true);
    setErrorMessage(null);
    try {
      const res = await getMyResumeUrl();
      const targetUrl = res.download_url || res.url;
      if (targetUrl) {
        window.open(targetUrl, "_blank", "noopener,noreferrer");
      } else {
        setErrorMessage("Presigned download link was empty. Please try again.");
      }
    } catch (err: unknown) {
      if (err instanceof ApiClientError && err.isNotFound) {
        setErrorMessage("Resume not found in storage. Please upload a new one.");
      } else {
        setErrorMessage("Failed to generate secure download link. Please try again.");
      }
    } finally {
      setIsDownloading(false);
    }
  };

  return (
    <div className="space-y-4">
      {/* Institutional Email Warning Banner */}
      {!isVerified && (
        <div className="rounded-xl border border-pastel-yellowText/20 bg-pastel-yellow p-4 text-xs text-pastel-yellowText space-y-2">
          <div className="flex items-center gap-2 font-semibold">
            <ShieldAlert className="h-4 w-4 shrink-0" />
            <span>Institutional .edu Verification Required</span>
          </div>
          <p className="leading-relaxed">
            Resume uploads are restricted to verified university students. Please verify your
            institutional email to unlock resume storage and proposal submission.
          </p>
          <div>
            <Link
              href="/verify-email"
              className="inline-flex items-center gap-1 font-medium underline underline-offset-4 hover:opacity-80"
            >
              Verify Email Address
            </Link>
          </div>
        </div>
      )}

      {/* Success Notification */}
      {successMessage && (
        <div className="flex items-center gap-2.5 rounded-lg border border-pastel-greenText/20 bg-pastel-green px-4 py-3 text-xs text-pastel-greenText">
          <CheckCircle2 className="h-4 w-4 shrink-0" />
          <span>{successMessage}</span>
        </div>
      )}

      {/* Error Alert */}
      {errorMessage && (
        <div className="flex items-center gap-2.5 rounded-lg border border-pastel-redText/20 bg-pastel-red px-4 py-3 text-xs text-pastel-redText">
          <AlertCircle className="h-4 w-4 shrink-0" />
          <span>{errorMessage}</span>
        </div>
      )}

      {/* Hidden File Input */}
      <input
        ref={fileInputRef}
        type="file"
        accept=".pdf,.docx,application/pdf,application/vnd.openxmlformats-officedocument.wordprocessingml.document"
        className="hidden"
        onChange={handleFileChange}
        disabled={!isVerified || isUploading}
      />

      {/* State 1: Resume is currently uploaded and not in override upload mode */}
      {hasResume && !showUploaderOverride ? (
        <div className="rounded-xl border border-border bg-card p-5 shadow-none space-y-4">
          <div className="flex items-start gap-3.5">
            <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-muted text-foreground">
              <FileText className="h-5 w-5" />
            </div>
            <div className="min-w-0 flex-1">
              <h4 className="font-serif text-base font-medium text-foreground truncate">
                {currentResume.filename || "Student_Resume.pdf"}
              </h4>
              <div className="flex flex-wrap items-center gap-2 mt-1 text-xs text-muted-foreground font-mono">
                <span>{formatFileSize(currentResume.byteSize)}</span>
                <span>-</span>
                <span>Uploaded {formatJobDate(currentResume.updatedAt)}</span>
              </div>
              <div className="mt-2">
                <Badge variant="default" className="text-[10px]">
                  Vault Active
                </Badge>
              </div>
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-2 pt-3 border-t border-border">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={handleDownloadResume}
              disabled={isDownloading}
              className="text-xs"
            >
              {isDownloading ? (
                <>
                  <Loader2 className="h-3.5 w-3.5 animate-spin mr-1.5" />
                  <span>Preparing...</span>
                </>
              ) : (
                <>
                  <Download className="h-3.5 w-3.5 mr-1.5" />
                  <span>Download / View</span>
                  <ExternalLink className="h-3 w-3 ml-1 opacity-60" />
                </>
              )}
            </Button>

            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={() => setShowUploaderOverride(true)}
              disabled={!isVerified}
              className="text-xs"
            >
              <RefreshCw className="h-3.5 w-3.5 mr-1.5" />
              <span>Replace Resume</span>
            </Button>
          </div>

          <div className="pt-2 border-t border-border flex items-center justify-between text-[11px] text-muted-foreground font-mono">
            <span>Secure presigned S3 URLs generate on-demand.</span>
            <span>Max 10MB - PDF/DOCX</span>
          </div>
        </div>
      ) : (
        /* State 2: Drag & Drop Uploader Area */
        <div
          onDragOver={handleDragOver}
          onDragLeave={handleDragLeave}
          onDrop={handleDrop}
          onClick={() => {
            if (isVerified && !isUploading) {
              fileInputRef.current?.click();
            }
          }}
          className={`relative flex flex-col items-center justify-center rounded-xl border-2 border-dashed p-8 text-center transition-all shadow-none ${
            !isVerified
              ? "cursor-not-allowed border-border bg-muted/40 opacity-70"
              : isDragging
                ? "cursor-copy border-foreground/60 bg-muted/30"
                : "cursor-pointer border-border bg-card hover:border-foreground/30"
          }`}
        >
          <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-muted text-foreground">
            {isUploading ? (
              <Loader2 className="h-6 w-6 animate-spin" />
            ) : (
              <Upload className="h-6 w-6" />
            )}
          </div>

          <div className="mt-4 space-y-1">
            <h4 className="font-serif text-base font-medium text-foreground">
              {isUploading
                ? "Uploading to private vault..."
                : isDragging
                  ? "Drop resume file here"
                  : "Upload your resume document"}
            </h4>
            <p className="text-xs text-muted-foreground font-mono">
              PDF or Word (.docx) up to 10MB
            </p>
          </div>

          <div className="mt-4 flex flex-wrap items-center justify-center gap-2">
            <Button
              type="button"
              size="sm"
              disabled={!isVerified || isUploading}
              onClick={(e) => {
                e.stopPropagation();
                if (isVerified && !isUploading) {
                  fileInputRef.current?.click();
                }
              }}
              className="text-xs"
            >
              {isUploading ? (
                <>
                  <Loader2 className="h-3.5 w-3.5 animate-spin mr-1.5" />
                  <span>Uploading...</span>
                </>
              ) : (
                <>
                  <Upload className="h-3.5 w-3.5 mr-1.5" />
                  <span>Choose File</span>
                </>
              )}
            </Button>

            {hasResume && showUploaderOverride && (
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={(e) => {
                  e.stopPropagation();
                  setShowUploaderOverride(false);
                  setErrorMessage(null);
                }}
                className="text-xs"
              >
                Keep Current Resume
              </Button>
            )}
          </div>

          {!isVerified && (
            <p className="mt-3 font-mono text-[11px] text-pastel-yellowText">
              Uploader locked until university email is verified.
            </p>
          )}
        </div>
      )}
    </div>
  );
}
