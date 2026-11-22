<?php

namespace Database\Factories;

use App\Domain\Pipeline;
use App\Models\Job;
use Illuminate\Database\Eloquent\Factories\Factory;

/** @extends Factory<Job> */
class JobFactory extends Factory
{
    protected $model = Job::class;

    public function definition(): array
    {
        return [
            'title' => fake()->jobTitle(),
            'unit' => fake()->city(),
            'status' => 'open',
            'stages' => Pipeline::DEFAULT_STAGES,
        ];
    }
}
