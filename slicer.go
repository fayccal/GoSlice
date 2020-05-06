package main

import (
	"GoSlice/clip"
	"GoSlice/data"
	"GoSlice/gcode"
	"GoSlice/gcode/renderer"
	"GoSlice/handle"
	"GoSlice/modify"
	"GoSlice/optimize"
	"GoSlice/slice"
	"GoSlice/stl"
	"GoSlice/write"
	"fmt"
	"time"
)

type GoSlice struct {
	o         *data.Options
	reader    handle.ModelReader
	optimizer handle.ModelOptimizer
	slicer    handle.ModelSlicer
	modifiers []handle.LayerModifier
	generator handle.GCodeGenerator
	writer    handle.GCodeWriter
}

func NewGoSlice(o ...option) *GoSlice {
	options := data.Options{
		Print: data.PrintOptions{
			IntialLayerSpeed:    30,
			LayerSpeed:          60,
			OuterPerimeterSpeed: 40,

			InitialLayerThickness: 200,
			LayerThickness:        200,
			InsetCount:            2,
			InfillOverlapPercent:  50,
			InfillPercent:         20,
		},
		Filament: data.FilamentOptions{
			FilamentDiameter: data.Millimeter(1.75).ToMicrometer(),
		},
		Printer: data.PrinterOptions{
			ExtrusionWidth: 400,
			Center: data.NewMicroVec3(
				data.Millimeter(100).ToMicrometer(),
				data.Millimeter(100).ToMicrometer(),
				0,
			),
		},

		MeldDistance:              30,
		JoinPolygonSnapDistance:   100,
		FinishPolygonSnapDistance: 1000,
	}

	topBottomPatternFactory := func(min data.MicroPoint, max data.MicroPoint) clip.Pattern {
		return clip.NewLinearPattern(min, max, options.Printer.ExtrusionWidth)
	}

	s := &GoSlice{
		o:         &options,
		reader:    stl.Reader(),
		optimizer: optimize.NewOptimizer(&options),
		slicer:    slice.NewSlicer(&options),
		modifiers: []handle.LayerModifier{
			modify.NewPerimeterModifier(&options),
			modify.NewInfillModifier(&options),
		},
		generator: gcode.NewGenerator(
			&options,
			gcode.WithRenderer(renderer.PreLayer{}),
			gcode.WithRenderer(renderer.Perimeter{}),
			gcode.WithRenderer(&renderer.Infill{
				PatternSetup: topBottomPatternFactory,
				AttrName:     "bottom",
				Comments:     []string{"TYPE:FILL", "BOTTOM-FILL"},
			}),
			gcode.WithRenderer(&renderer.Infill{
				PatternSetup: topBottomPatternFactory,
				AttrName:     "top",
				Comments:     []string{"TYPE:FILL", "TOP-FILL"},
			}),
			gcode.WithRenderer(&renderer.Infill{
				PatternSetup: func(min data.MicroPoint, max data.MicroPoint) clip.Pattern {
					// TODO: the calculation of the percentage is currently very basic and may not be correct.

					if options.Print.InfillPercent != 0 {
						mm10 := data.Millimeter(10).ToMicrometer()
						linesPer10mmFor100Percent := mm10 / options.Printer.ExtrusionWidth
						linesPer10mmForInfillPercent := float64(linesPer10mmFor100Percent) * float64(options.Print.InfillPercent) / 100.0

						lineWidth := data.Micrometer(float64(mm10) / linesPer10mmForInfillPercent)

						return clip.NewLinearPattern(min, max, lineWidth)
					}

					return nil
				},
				AttrName: "infill",
				Comments: []string{"TYPE:FILL", "INTERNAL-FILL"},
			}),
		),
		writer: write.Writer(),
	}

	s.With(o...)

	return s
}

func (s *GoSlice) Process(filename string, outFilename string) error {
	startTime := time.Now()

	// 1. Load model
	models, err := s.reader.Read(filename)
	if err != nil {
		return err
	}

	// 2. Optimize model
	var optimizedModel data.OptimizedModel

	// TODO: support several model processing
	//for i, model := range models {
	optimizedModel, err = s.optimizer.Optimize(models[0])
	if err != nil {
		return err
	}
	//}

	optimizedModel.SaveDebugSTL("test.stl")

	// 3. Slice model into layers
	layers, err := s.slicer.Slice(optimizedModel)
	if err != nil {
		return err
	}

	// 4. Modify the layers
	// e.g. classify them,
	// generate the parts which should be filled in,
	// generate perimeter paths
	for _, m := range s.modifiers {
		m.Init(optimizedModel)
		for layerNr := range layers {
			layers, err = m.Modify(layerNr, layers)
			if err != nil {
				return err
			}
		}
	}

	// 5. generate gcode from the layers
	s.generator.Init(optimizedModel)
	gcode := s.generator.Generate(layers)

	err = s.writer.Write(gcode, outFilename)

	fmt.Println("full processing time:", time.Now().Sub(startTime))

	return nil
}
