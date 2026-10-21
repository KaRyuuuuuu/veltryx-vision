import { Camera } from "../services/api";
import CameraViewer from "./CameraViewer";
export default function CameraGrid({
  cameras,
  onEdit,
  onShot,
  onError,
}: {
  cameras: Camera[];
  onEdit: (c: Camera) => void;
  onShot: () => void;
  onError: (s: string) => void;
}) {
  return (
    <div className="camera-grid">
      {cameras.map((c) => (
        <CameraViewer
          key={c.serial}
          camera={c}
          onEdit={() => onEdit(c)}
          onShot={onShot}
          onError={onError}
        />
      ))}
    </div>
  );
}
