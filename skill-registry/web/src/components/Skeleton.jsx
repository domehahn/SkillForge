export function SkeletonCards({ count = 5 }) {
  return (
    <div className="artifacts-list">
      {Array.from({ length: count }).map((_, i) => (
        <div key={i} className="skeleton-card">
          <div className="skel skeleton-card-avatar" />
          <div className="skeleton-card-body">
            <div className="skel" style={{ width: '50%', height: 15, marginBottom: 8 }} />
            <div className="skel" style={{ width: '30%', height: 12, marginBottom: 12 }} />
            <div className="skel" style={{ width: '92%', height: 12, marginBottom: 5 }} />
            <div className="skel" style={{ width: '70%', height: 12, marginBottom: 14 }} />
            <div style={{ display: 'flex', gap: 8 }}>
              <div className="skel" style={{ width: 48, height: 20, borderRadius: 4 }} />
              <div className="skel" style={{ width: 48, height: 20, borderRadius: 4 }} />
            </div>
          </div>
        </div>
      ))}
    </div>
  )
}

export function SkeletonDetail() {
  return (
    <div className="skeleton-detail-wrap">
      {/* breadcrumb */}
      <div className="skel" style={{ width: 200, height: 14, marginBottom: 20 }} />
      {/* hero */}
      <div style={{ display: 'flex', gap: 20, marginBottom: 32 }}>
        <div className="skel" style={{ width: 72, height: 72, borderRadius: 14, flexShrink: 0 }} />
        <div style={{ flex: 1 }}>
          <div className="skel" style={{ width: '40%', height: 28, marginBottom: 12 }} />
          <div className="skel" style={{ width: '28%', height: 14, marginBottom: 10 }} />
          <div className="skel" style={{ width: '65%', height: 14, marginBottom: 10 }} />
          <div className="skel" style={{ width: '45%', height: 13 }} />
        </div>
      </div>
      {/* tabs + content */}
      <div style={{ display: 'flex', gap: 28 }}>
        <div style={{ flex: 1 }}>
          <div className="skel" style={{ height: 38, borderRadius: 8, marginBottom: 20 }} />
          <div className="skel" style={{ height: 100, borderRadius: 8, marginBottom: 14 }} />
          <div className="skel" style={{ height: 14, width: '90%', marginBottom: 8 }} />
          <div className="skel" style={{ height: 14, width: '75%', marginBottom: 8 }} />
          <div className="skel" style={{ height: 14, width: '82%', marginBottom: 8 }} />
          <div className="skel" style={{ height: 14, width: '60%' }} />
        </div>
        <div style={{ width: 300, flexShrink: 0 }}>
          <div className="skel" style={{ height: 90, borderRadius: 10, marginBottom: 14 }} />
          <div className="skel" style={{ height: 130, borderRadius: 10, marginBottom: 14 }} />
          <div className="skel" style={{ height: 90, borderRadius: 10 }} />
        </div>
      </div>
    </div>
  )
}

export function SkeletonGridCards({ count = 8 }) {
  return (
    <div className="artifacts-grid">
      {Array.from({ length: count }).map((_, i) => (
        <div key={i} className="skeleton-grid-card">
          <div className="skel skeleton-grid-avatar" />
          <div className="skel" style={{ width: '70%', height: 14, marginBottom: 8 }} />
          <div className="skel" style={{ width: '45%', height: 11, marginBottom: 10 }} />
          <div className="skel" style={{ width: '95%', height: 11, marginBottom: 5 }} />
          <div className="skel" style={{ width: '80%', height: 11 }} />
        </div>
      ))}
    </div>
  )
}
