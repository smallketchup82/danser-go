package movers

import (
	"github.com/wieku/danser-go/app/beatmap/difficulty"
	"github.com/wieku/danser-go/app/beatmap/objects"
	"github.com/wieku/danser-go/app/bmath"
	"github.com/wieku/danser-go/app/settings"
	"github.com/wieku/danser-go/framework/math/curves"
	"github.com/wieku/danser-go/framework/math/math32"
	"github.com/wieku/danser-go/framework/math/vector"
	"math"
)

// https://github.com/TechnoJo4/osu/blob/master/osu.Game.Rulesets.Osu/Replays/Movers/MomentumMover.cs

type MomentumMover struct {
	bz        *curves.Bezier
	last      vector.Vector2f
	startTime int64
	endTime   int64
	first     bool
	wasStream bool
	mods      difficulty.Modifier
}

func NewMomentumMover() MultiPointMover {
	return &MomentumMover{last: vector.NewVec2f(0, 0), first: true}
}

func (bm *MomentumMover) Reset(mods difficulty.Modifier) {
	bm.mods = mods
	bm.first = true
	bm.last = vector.NewVec2f(0, 0)
}

func same(mods difficulty.Modifier, o1 objects.IHitObject, o2 objects.IHitObject) bool {
	return o1.GetStackedStartPositionMod(mods) == o2.GetStackedStartPositionMod(mods) || (settings.Dance.Momentum.SkipStackAngles && o1.GetStartPosition() == o2.GetStartPosition())
}

func anorm(a float32) float32 {
	pi2 := 2 * math32.Pi
	a = math32.Mod(a, pi2)
	if a < 0 {
		a += pi2
	}

	return a
}

func anorm2(a float32) float32 {
	a = anorm(a)
	if a > math32.Pi {
		a = -(2 * math32.Pi - a)
	}

	return a
}

func (bm *MomentumMover) SetObjects(objs []objects.IHitObject) int {
	i := 0

	end := objs[i+0]
	start := objs[i+1]

	hasNext := false
	var next objects.IHitObject
	if len(objs) > 2 {
		if _, ok := objs[i+2].(*objects.Circle); ok {
			hasNext = true
			next = objs[i+2]
		} else if v, ok := objs[i+2].(*objects.Slider); ok && v.IsRetarded() {
			hasNext = true
			next = objs[i+2]
		}
	}

	endPos := end.GetStackedEndPositionMod(bm.mods)
	startPos := start.GetStackedStartPositionMod(bm.mods)

	dst := endPos.Dst(startPos)

	var a2 float32
	fromSlider := false
	for i++; i < len(objs); i++ {
		o := objs[i]
		if s, ok := o.(*objects.Slider); ok && !s.IsRetarded() {
			a2 = s.GetStartAngleMod(bm.mods)
			fromSlider = true
			break
		}
		if i == len(objs) - 1 {
			a2 = bm.last.AngleRV(endPos)
			break
		}
		if !same(bm.mods, o, objs[i+1]) {
			a2 = o.GetStackedStartPositionMod(bm.mods).AngleRV(objs[i+1].GetStackedStartPositionMod(bm.mods))
			break
		}
	}

	s, ok1 := end.(*objects.Slider)
	if ok1 {
		ok1 = !s.IsRetarded()
	}

	ms := settings.Dance.Momentum

	// stream detection logic stolen from spline mover
	stream := false
	if hasNext && !fromSlider && ms.StreamRestrict {
		min := float32(25.0)
		max := float32(10000.0)
		nextPos := next.GetStackedStartPositionMod(bm.mods)
		sq1 := endPos.DstSq(startPos)
		sq2 := startPos.DstSq(nextPos)

		if sq1 >= min && sq1 <= max && bm.wasStream || (sq2 >= min && sq2 <= max) {
			stream = true
		}
	}

	bm.wasStream = stream

	var a1 float32
	if s, ok := end.(*objects.Slider); ok {
		a1 = s.GetEndAngleMod(bm.mods)
	} else if bm.first {
		a1 = a2 + math.Pi
	} else {
		a1 = endPos.AngleRV(bm.last)
	}

	offset := float32(ms.RestrictAngle * math.Pi / 180.0)

	multEnd := ms.DistanceMultOut
	multStart := ms.DistanceMultOut

	if stream && math32.Abs(anorm(a2 - startPos.AngleRV(endPos))) < anorm((2 * math32.Pi) - offset) {
		a := endPos.AngleRV(startPos)
		sangle := float32(ms.StreamAngle * math.Pi / 180.0)
		if anorm(a1 - a) > math32.Pi {
			a2 = a - sangle
		} else {
			a2 = a + sangle
		}

		multEnd = ms.StreamMult
		multStart = ms.StreamMult
	} else if !fromSlider && math32.Abs(anorm2(a2 - startPos.AngleRV(endPos))) < offset {
		a := startPos.AngleRV(endPos)
		if anorm(a2 - a) < offset {
			a2 = a - offset
		} else {
			a2 = a + offset
		}

		multEnd = ms.DistanceMult
		multStart = ms.DistanceMultEnd
	}

	endTime := end.GetEndTime()
	startTime := start.GetStartTime()
	duration := float64(startTime - endTime)

	if ms.DurationTrigger > 0 && duration >= ms.DurationTrigger {
		mult := math.Pow(ms.DurationPow, float64(duration) / ms.DurationTrigger)
		multEnd *= mult
		multStart *= mult
	}

	p1 := vector.NewVec2fRad(a1, dst * float32(multEnd)).Add(endPos)
	p2 := vector.NewVec2fRad(a2, dst * float32(multStart)).Add(startPos)

	if !same(bm.mods, end, start) {
		bm.last = p2
	}

	bm.bz = curves.NewBezierNA([]vector.Vector2f{endPos, p1, p2, startPos})
	bm.endTime = end.GetEndTime()
	bm.startTime = start.GetStartTime()
	bm.first = false

	return 2
}

func (bm *MomentumMover) Update(time int64) vector.Vector2f {
	t := bmath.ClampF32(float32(time-bm.endTime)/float32(bm.startTime-bm.endTime), 0, 1)
	return bm.bz.PointAt(t)
}

func (bm *MomentumMover) GetEndTime() int64 {
	return bm.startTime
}
